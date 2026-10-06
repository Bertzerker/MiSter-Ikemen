// Host-side tests of the actual C conversion and publication code, using RAM.
#include "../internal/frontiervideo/transport.c"
#include <assert.h>
#include <stdlib.h>
#include <stdio.h>

int main(void) {
    uint8_t source[24] = {255,0,0,255, 0,255,0,0, 99,99,99,99,
                         0,0,255,255, 255,255,255,255, 99,99,99,99};
    uint16_t converted[4] = {0};
    frontier_convert(converted, source, 2, 2, 12);
    assert(converted[0] == 0xf800 && converted[1] == 0x07e0);
    assert(converted[2] == 0x001f && converted[3] == 0xffff);
    memory = calloc(1, MAP_SIZE);
    assert(memory);
    const unsigned joy_offsets[] = {0x08, 0x18, 0x20, 0x28};
    for (int p = 0; p < 4; p++) {
        *(volatile uint32_t *)(memory + joy_offsets[p]) = 1u << (p + 4);
        assert(frontier_joystick(p) == 1u << (p + 4));
    }
    assert(frontier_joystick(-1) == 0 && frontier_joystick(4) == 0);
    uint8_t *frame = calloc(WIDTH * HEIGHT, 4);
    assert(frame);
    for (unsigned i = 0; i < WIDTH * HEIGHT; ++i) frame[i * 4] = 255;
    *(volatile uint32_t *)(memory + ACK + 4) = MAGIC;
    *(volatile uint32_t *)(memory + ACK) = 0;
    published = 0;
    assert(frontier_present(frame, WIDTH, HEIGHT, WIDTH * 4) == 0);
    assert(read_word(0) == 5); // sequence 1, buffer 1
    assert(*(uint16_t *)(memory + 0x40) == 0); // buffer 0 untouched
    assert(*(uint16_t *)(memory + 0x40040 + (WIDTH * HEIGHT - 1) * 2) == 0xf800);
    // Cannot lap the reader, even if rendering is much faster than the display.
    assert(frontier_present(frame, WIDTH, HEIGHT, WIDTH * 4) == ETIMEDOUT);
    assert(read_word(0) == 5);
    *(volatile uint32_t *)(memory + ACK) = 5;
    assert(frontier_present(frame, WIDTH, HEIGHT, WIDTH * 4) == 0);
    assert(read_word(0) == 8); // sequence 2, buffer 0
    assert(frontier_present(frame, WIDTH, 224, WIDTH * 4) == EINVAL);
    assert(frontier_present(NULL, WIDTH, HEIGHT, WIDTH * 4) == EINVAL);
    // An old video core must not activate a stale GPU mailbox left in DDR.
    *(volatile uint32_t *)(memory + GPU_MAILBOX + 20) = GPU_MAGIC;
    assert(frontier_gpu_open() == ENODEV);
    gpu_memory = calloc(1, GPU_SIZE);
    assert(gpu_memory);
    assert(!frontier_gpu_grouped_rows());
    assert(valid_gpu_magic(GPU_MAGIC) && valid_gpu_magic(GPU_GROUP_MAGIC));
    assert(!valid_gpu_magic(0));
    *(volatile uint32_t *)(memory + GPU_MAILBOX + 20) = GPU_GROUP_MAGIC;
    assert(frontier_gpu_grouped_rows());
    *(volatile uint32_t *)(memory + GPU_MAILBOX + 20) = GPU_DEFER_MAGIC;
    assert(valid_gpu_magic(GPU_DEFER_MAGIC) && frontier_gpu_grouped_rows());
    uint8_t texture[64] = {19, 37, 53, 255};
    assert(frontier_gpu_upload(0x38400000u, texture, sizeof(texture)) == 0);
    assert(memcmp(gpu_memory + 0x400000u, texture, sizeof(texture)) == 0);
    assert(frontier_gpu_upload(0x39bfffc0u, texture, sizeof(texture)) == 0);
    assert(frontier_gpu_upload(0x383fffffu, texture, 1) == EINVAL);
    assert(frontier_gpu_upload(0x39bfffc1u, texture, sizeof(texture)) == EINVAL);
    assert(frontier_gpu_upload(0xfffffff0u, texture, sizeof(texture)) == EINVAL);
    assert(frontier_gpu_frame(texture, 63, NULL, 0) == EINVAL);
    assert(frontier_gpu_frame(texture, 0x200040u, NULL, 0) == EINVAL);
    assert(frontier_gpu_frame(NULL, 64, NULL, 0) == EINVAL);
    assert(frontier_gpu_frame(NULL, 0, texture, 0x200001u) == EINVAL);
    assert(frontier_gpu_readback(frame, WIDTH*HEIGHT*4-1) == EINVAL);
    memcpy(gpu_memory + 0x1c00000u, texture, sizeof(texture));
    assert(frontier_gpu_readback(frame, WIDTH*HEIGHT*4) == 0);
    assert(memcmp(frame, texture, sizeof(texture)) == 0);
    free(gpu_memory);
    gpu_memory = NULL;
    free(frame);
    free((void *)memory);
    memory = NULL;
    puts("PASS: RGB565 channels/stride, complete 240-line frame, alternating buffers, ACK timeout, invalid dimensions");
    puts("PASS: stale GPU refusal, texture arena bounds/overflow, command limits and RGBA readback");
    return 0;
}
