// Read-only publication-rate measurement. SPDX-License-Identifier: MIT
#define _DEFAULT_SOURCE
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/mman.h>
#include <time.h>
#include <unistd.h>
static double now(void) {
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return t.tv_sec + t.tv_nsec / 1e9;
}
int main(int argc, char **argv) {
    int seconds = argc > 1 ? atoi(argv[1]) : 20;
    if (seconds < 1 || seconds > 120) return 2;
    int fd = open("/dev/mem", O_RDONLY | O_SYNC);
    if (fd < 0) { perror("open"); return 1; }
    volatile uint32_t *m = mmap(NULL, 0x100000, PROT_READ, MAP_SHARED, fd, 0x3a000000);
    close(fd);
    if (m == MAP_FAILED) { perror("mmap"); return 1; }
    if (m[0xe0004/4] != 0x494b5631 && m[0xe0004/4] != 0x494b5632) return 1;
    uint32_t previous = m[0] >> 2, frames = 0;
    uint32_t gpu_previous = m[0xe0110/4], gpu_samples = 0;
    uint64_t gpu_cycles = 0, gpu_commands = 0;
    double start = now(), changed = start, longest = 0, elapsed;
    do {
        usleep(5000);
        uint32_t current = m[0] >> 2;
        double t = now();
        uint32_t delta = (current - previous) & 0x3fffffff;
        if (delta > 1000) { fprintf(stderr, "core reset during measurement\n"); return 1; }
        if (delta) {
            frames += delta;
            previous = current;
            changed = t;
        }
        if (t - changed > longest) longest = t - changed;
        elapsed = t - start;
        // Sample completed GPU work without profiling the game. Only pair
        // command count with cycles while that request is still acknowledged.
        uint32_t magic = m[0xe0114/4], ack = m[0xe0110/4];
        if ((magic == 0x494b4731 || magic == 0x494b4732 || magic == 0x494b4733) && ack != gpu_previous && m[0xe0100/4] == ack) {
            uint32_t cycles = m[0xe011c/4], commands = m[0xe0104/4];
            if (m[0xe0100/4] == ack && m[0xe0118/4] == 0) {
                gpu_cycles += cycles;gpu_commands += commands;gpu_samples++;gpu_previous = ack;
            }
        }
    } while (elapsed < seconds);
    printf("%u frames / %.3f seconds = %.2f fps; longest observed publication pause %.1f ms\n",
           frames, elapsed, frames / elapsed, longest * 1000);
    if (gpu_samples) printf("GPU completed samples=%u; average %.2f ms at 98.4375 MHz; %.0f commands/frame\n",
        gpu_samples, (double)gpu_cycles/gpu_samples/98437.5, (double)gpu_commands/gpu_samples);
    munmap((void *)m, 0x100000);
    return 0;
}
