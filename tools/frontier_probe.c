// Read-only protocol probe. SPDX-License-Identifier: MIT
#define _DEFAULT_SOURCE
#include <fcntl.h>
#include <stdint.h>
#include <stdio.h>
#include <sys/mman.h>
#include <unistd.h>
int main(void) {
 int fd=open("/dev/mem",O_RDONLY|O_SYNC);
 if(fd<0) {perror("/dev/mem");return 1;}
 volatile uint32_t *m=mmap(NULL,0x100000,PROT_READ,MAP_SHARED,fd,0x3a000000);
 close(fd);
 if(m==MAP_FAILED) {perror("mmap");return 1;}
 for(int i=0;i<5;i++) {
  printf("control=%08x ack=%08x magic=%08x heartbeat=%u heartbeat_magic=%08x\n",
    m[0],m[0xe0000/4],m[0xe0004/4],m[0xe0008/4],m[0xe000c/4]);
  if(m[0xe0004/4]==0x494b5632)
   printf("gpu request=%u count=%u done=%u magic=%08x error=%u cycles=%u\n",
    m[0xe0100/4],m[0xe0104/4],m[0xe0110/4],m[0xe0114/4],m[0xe0118/4],m[0xe011c/4]);
  fflush(stdout);usleep(250000);
 }
 munmap((void*)m,0x100000);
 return 0;
}
