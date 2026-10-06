#ifndef IKEMEN_FRONTIER_TRANSPORT_H
#define IKEMEN_FRONTIER_TRANSPORT_H
#include <stdint.h>
#include <stddef.h>
int frontier_open(void);
int frontier_present(const uint8_t *rgba, int width, int height, int pitch);
void frontier_close(void);
uint32_t frontier_joystick(int player);
void frontier_convert(uint16_t *dst, const uint8_t *src, int width, int height, int pitch);
int frontier_gpu_open(void);
int frontier_gpu_grouped_rows(void);
int frontier_gpu_upload(uint32_t address, const uint8_t *data, size_t size);
int frontier_gpu_frame(const uint8_t *commands, size_t command_bytes, const uint8_t *palettes, size_t palette_bytes);
int frontier_gpu_readback(uint8_t *rgba, size_t size);
#endif
