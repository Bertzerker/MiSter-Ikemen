// IKEMEN Frontier video transport. SPDX-License-Identifier: MIT
#define _DEFAULT_SOURCE
#include "transport.h"
#include <errno.h>
#include <fcntl.h>
#include <string.h>
#include <sys/file.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
#include <stdio.h>
#include <linux/fb.h>
#include <sys/ioctl.h>

#define BASE 0x3A000000u
#define MAP_SIZE 0x100000u
#define ACK 0xE0000u
#define HEARTBEAT 0xE0008u
#define MAGIC 0x494B5631u
#define GPU_VIDEO_MAGIC 0x494B5632u
#define GPU_BASE 0x38000000u
#define GPU_SIZE 0x01d00000u
#define GPU_MAILBOX 0xE0100u
#define GPU_MAGIC 0x494B4731u
#define GPU_GROUP_MAGIC 0x494B4732u
#define GPU_DEFER_MAGIC 0x494B4733u
static int valid_gpu_magic(uint32_t magic) { return magic==GPU_MAGIC || magic==GPU_GROUP_MAGIC || magic==GPU_DEFER_MAGIC; }
#define WIDTH 320
#define HEIGHT 240
static volatile uint8_t *memory;
static int lock_fd = -1;
static uint32_t published;
static uint8_t *gpu_memory;
// Convert in cached ARM RAM, then copy in aligned bursts. Halfword stores
// directly into the uncached DDR mapping serialize every individual pixel.
static uint16_t converted[WIDTH * HEIGHT] __attribute__((aligned(16)));

static inline void barrier(void) { __sync_synchronize(); }
static uint32_t read_word(unsigned offset) {
    uint32_t value = *(volatile uint32_t *)(memory + offset);
    barrier();
    return value;
}
static uint64_t millis(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (uint64_t)t.tv_sec * 1000 + t.tv_nsec / 1000000;
}
static int valid_magic(uint32_t m) { return m == MAGIC || m == GPU_VIDEO_MAGIC; }

uint32_t frontier_joystick(int player) {
    static const unsigned offsets[] = {0x08, 0x18, 0x20, 0x28};
    if (!memory || player < 0 || player >= 4) return 0;
    return read_word(offsets[player]);
}

void frontier_convert(uint16_t *dst, const uint8_t *src, int width, int height, int pitch) {
    for (int y = 0; y < height; ++y) {
        const uint8_t *row = src + (size_t)y * pitch;
        for (int x = 0; x < width; ++x) {
            const uint8_t *p = row + x * 4;
            dst[y * width + x] = (uint16_t)(((p[0] & 248) << 8) |
                                           ((p[1] & 252) << 3) | (p[2] >> 3));
        }
    }
}

static int wait_ack(uint32_t expected) {
    uint64_t deadline = millis() + 250;
    do {
        if (valid_magic(read_word(ACK + 4)) && read_word(ACK) == expected) return 0;
        usleep(200);
    } while (millis() < deadline);
    return ETIMEDOUT;
}

void frontier_close(void) {
    // Do not clear published buffers: FPGA may still be scanning one.
    if (memory) munmap((void *)memory, MAP_SIZE);
    memory = NULL;
    if (gpu_memory) munmap(gpu_memory, GPU_SIZE);
    gpu_memory = NULL;
    if (lock_fd >= 0) close(lock_fd);
    lock_fd = -1;
}

int frontier_open(void) {
    if (memory) return EBUSY;
    lock_fd = open("/tmp/ikemen-frontier-video.lock", O_CREAT | O_RDWR, 0600);
    if (lock_fd < 0) return errno;
    if (flock(lock_fd, LOCK_EX | LOCK_NB)) { int e = errno; frontier_close(); return e; }
    int fd = open("/dev/mem", O_RDWR | O_SYNC);
    if (fd < 0) { int e = errno; frontier_close(); return e; }
    void *mapped = mmap(NULL, MAP_SIZE, PROT_READ | PROT_WRITE, MAP_SHARED, fd, BASE);
    int saved = errno;
    close(fd);
    if (mapped == MAP_FAILED) { frontier_close(); return saved; }
    memory = mapped;

    // A changing heartbeat is required: stale RAM from an old core is insufficient.
    uint32_t initial = read_word(HEARTBEAT);
    uint64_t deadline = millis() + 500;
    int changes = 0;
    do {
        uint32_t current = read_word(HEARTBEAT);
        if (valid_magic(read_word(HEARTBEAT + 4)) && current != initial) {
            initial = current;
            if (++changes >= 2) break;
        }
        usleep(1000);
    } while (millis() < deadline);
    if (changes < 2) { frontier_close(); return ENODEV; }
    published = read_word(0);
    int error = wait_ack(published);
    if (error) frontier_close();
    return error;
}

int frontier_present(const uint8_t *rgba, int width, int height, int pitch) {
    if (!memory) return ENODEV;
    if (!rgba || width != WIDTH || height != HEIGHT || pitch < WIDTH * 4) return EINVAL;
    int error = wait_ack(published);
    if (error) return error;
    // ACK marks the buffer latched by FPGA. Never write that buffer.
    unsigned next = (published & 1u) ^ 1u;
    uint16_t *dst = (uint16_t *)(memory + 0x40u + next * 0x40000u);
    frontier_convert(converted, rgba, width, height, pitch);
    memcpy(dst, converted, sizeof(converted));
    barrier();
    published = ((published + 4u) & ~3u) | next;
    *(volatile uint32_t *)memory = published;
    barrier();
    return 0;
}

int frontier_gpu_open(void) {
    if (!memory || read_word(ACK+4) != GPU_VIDEO_MAGIC || !valid_gpu_magic(read_word(GPU_MAILBOX+20))) return ENODEV;
    // Refuse to map an arena overlapping Linux RAM or the runtime HPS framebuffer.
    FILE *maps = fopen("/proc/iomem", "r");
    if (!maps) return errno;
    char line[256]; unsigned long long start, end; int seen_ram = 0;
    while (fgets(line, sizeof(line), maps)) {
        if (strstr(line, "System RAM") && sscanf(line, "%llx-%llx", &start, &end) == 2) {
            if (end > 0) seen_ram = 1;
            if (start < GPU_BASE+GPU_SIZE && end >= GPU_BASE) { fclose(maps); return EBUSY; }
        }
    }
    fclose(maps);
    if (!seen_ram) return EACCES;
    int fb = open("/dev/fb0", O_RDONLY);
    if (fb >= 0) {
        struct fb_fix_screeninfo info;
        if (ioctl(fb, FBIOGET_FSCREENINFO, &info)) { int e=errno; close(fb); return e; }
        close(fb);
        if (info.smem_len && info.smem_start < GPU_BASE+GPU_SIZE &&
            (unsigned long long)info.smem_start+info.smem_len > GPU_BASE) return EBUSY;
    } else if (errno != ENOENT && errno != ENODEV) return errno;
    int fd=open("/dev/mem",O_RDWR|O_SYNC);
    if(fd<0) return errno;
    void *mapped=mmap(NULL,GPU_SIZE,PROT_READ|PROT_WRITE,MAP_SHARED,fd,GPU_BASE);
    int saved=errno;close(fd);
    if(mapped==MAP_FAILED)return saved;
    gpu_memory=mapped;
    return 0;
}

int frontier_gpu_grouped_rows(void) {
    if(!gpu_memory)return 0;
    uint32_t magic=read_word(GPU_MAILBOX+20);
    return magic==GPU_GROUP_MAGIC || magic==GPU_DEFER_MAGIC;
}

int frontier_gpu_upload(uint32_t address, const uint8_t *data, size_t size) {
    if(!gpu_memory)return ENODEV;
    if(!data || address<0x38400000u || address>=0x39c00000u || size>0x39c00000u-address)return EINVAL;
    memcpy(gpu_memory+(address-GPU_BASE),data,size);
    barrier();return 0;
}

int frontier_gpu_frame(const uint8_t *commands, size_t command_bytes, const uint8_t *palettes, size_t palette_bytes) {
    if(!gpu_memory)return ENODEV;
    if(command_bytes>0x200000u || command_bytes%64 || palette_bytes>0x200000u ||
       (command_bytes && !commands) || (palette_bytes && !palettes))return EINVAL;
    // IKG3 checks ownership after drawing, immediately before export. Older
    // cores retain the ARM-side wait. Never remove both sides of this fence.
    int deferred=read_word(GPU_MAILBOX+20)==GPU_DEFER_MAGIC;
    if(!deferred) { int error=wait_ack(published);if(error)return error; }
    if(command_bytes)memcpy(gpu_memory,commands,command_bytes);
    if(palette_bytes)memcpy(gpu_memory+0x200000u,palettes,palette_bytes);
    unsigned next=(published&1u)^1u;
    uint32_t request=read_word(GPU_MAILBOX+16)+1u;
    *(volatile uint32_t *)(memory+GPU_MAILBOX+8)=next | (deferred?2u:0u);
    *(volatile uint32_t *)(memory+GPU_MAILBOX+12)=deferred?published:0;
    *(volatile uint32_t *)(memory+GPU_MAILBOX+4)=(uint32_t)(command_bytes/64);
    barrier();
    *(volatile uint32_t *)(memory+GPU_MAILBOX)=request;
    barrier();
    uint64_t deadline=millis()+2000;
    while(read_word(GPU_MAILBOX+16)!=request) {
        if(millis()>=deadline)return ETIMEDOUT;
        usleep(200);
    }
    if(!valid_gpu_magic(read_word(GPU_MAILBOX+20)) || read_word(GPU_MAILBOX+24)!=0)return EIO;
    published=((published+4u)&~3u)|next;
    *(volatile uint32_t *)memory=published;
    barrier();return 0;
}

int frontier_gpu_readback(uint8_t *rgba, size_t size) {
    if(!gpu_memory)return ENODEV;
    if(!rgba || size!=WIDTH*HEIGHT*4)return EINVAL;
    barrier();memcpy(rgba,gpu_memory+0x1c00000u,size);return 0;
}
