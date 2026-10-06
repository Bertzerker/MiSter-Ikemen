// Explicit, bounded board smoke test. Requires the experimental IKV2 core.
// Uses the production transport, checks every pixel, then releases its lock.
#define _POSIX_C_SOURCE 200809L
#include "../internal/frontiervideo/transport.h"
#include <stdio.h>
#include <string.h>
#include <time.h>

static uint32_t commands[242][16];
static uint8_t actual[320*240*4], expected[320*240*4];
static uint8_t indices[256], palette[2048], rgba[1024];
static unsigned mul255(unsigned a, unsigned b) { return (a*b+127)/255; }
static int checked(int error, const char *operation) {
    if(error) fprintf(stderr,"%s: %s (%d)\n",operation,strerror(error),error);
    return error;
}
int main(void) {
    int status=1;
    if(checked(frontier_open(),"video open")) return 1;
    if(checked(frontier_gpu_open(),"GPU open")) goto done;
    for(unsigned i=0;i<256;i++) {
        indices[i]=i;
        unsigned a=i, r=i, g=255-i, b=77;
        palette[i*8]=mul255(r,a); palette[i*8+1]=mul255(g,a);
        palette[i*8+2]=mul255(b,a); palette[i*8+3]=a;
        palette[i*8+4]=r;palette[i*8+5]=g;palette[i*8+6]=b;
        rgba[i*4]=r;rgba[i*4+1]=g;rgba[i*4+2]=b;rgba[i*4+3]=a;
    }
    if(checked(frontier_gpu_upload(0x38400000,indices,sizeof(indices)),"index upload") ||
       checked(frontier_gpu_upload(0x38400100,rgba,sizeof(rgba)),"RGBA upload")) goto done;
    printf("Grouped rows capability: %d\n",frontier_gpu_grouped_rows());
    for(unsigned frame=0;frame<6;frame++) {
        if(frame==3) {
            // Same DDR address, new bytes: the frame boundary must invalidate
            // cached texture lines before this upload can be sampled.
            for(unsigned i=0;i<256;i++) indices[i]^=128;
            if(checked(frontier_gpu_upload(0x38400000,indices,sizeof(indices)),"replacement index upload")) goto done;
        }
        memset(commands,0,sizeof(commands));memset(expected,0,sizeof(expected));
        commands[0][0]=3;commands[0][2]=320*240;
        // Background replace followed by alpha-over indexed or RGBA rows.
        commands[1][0]=7<<8;commands[1][2]=320*240;
        commands[1][11]=0xff382818;commands[1][12]=0x00382818;
        for(unsigned p=0;p<320*240;p++) {
            expected[p*4]=24;expected[p*4+1]=40;expected[p*4+2]=56;expected[p*4+3]=255;
        }
        for(unsigned y=0;y<240;y++) {
            uint32_t *c=commands[y+2];
            c[0]=(frame==1?2:1)|(3<<8);c[1]=y*320;c[2]=320;
            c[3]=frame==1?0x38400100:0x38400000;c[4]=256;c[5]=1;
            c[6]=frame==2?255<<16:0;c[8]=frame==2?(uint32_t)-65536:65536;c[10]=0x38200000;
            if(frame>=4) {
                c[0]=(frame==5?2:1)|(3<<8);c[3]=frame==5?0x38400100:0x38400000;
                c[4]=16;c[5]=16;c[7]=((y*7)%16)<<16;
            }
            for(unsigned x=0;x<320;x++) {
                unsigned index=frame==2?(x>255?0:255-x):(x>255?255:x);
                if(frame>=4) index=((y*7)%16)*16+(x>15?15:x);
                if(frame==3 || frame==4) index^=128;
                unsigned source[4]={index,255-index,77,index};
                for(unsigned channel=0;channel<4;channel++) {
                    unsigned p=(y*320+x)*4+channel,d=expected[p];
                    expected[p]=mul255(source[channel],index)+d-mul255(d,index);
                }
            }
        }
        unsigned count=242;
        if(frontier_gpu_grouped_rows()) {
            count=2;
            for(unsigned y=0;y<240;y+=4) {
                memcpy(commands[count],commands[y+2],64);
                commands[count][0]|=3<<4;
                for(unsigned row=1;row<4;row++) commands[count][12+row]=commands[y+2+row][7];
                count++;
            }
        }
        struct timespec start,end;clock_gettime(CLOCK_MONOTONIC,&start);
        if(checked(frontier_gpu_frame((uint8_t *)commands,count*64,palette,sizeof(palette)),"GPU frame") ||
           checked(frontier_gpu_readback(actual,sizeof(actual)),"GPU readback")) goto done;
        clock_gettime(CLOCK_MONOTONIC,&end);
        for(unsigned p=0;p<sizeof(actual);p++) if(actual[p]!=expected[p]) {
            fprintf(stderr,"frame %u pixel %u channel %u: got %u expected %u\n",frame,p/4,p%4,actual[p],expected[p]);goto done;
        }
        printf("PASS frame %u: all 76800 RGBA pixels, submit/readback %.2f ms\n",frame,
            (end.tv_sec-start.tv_sec)*1000.0+(end.tv_nsec-start.tv_nsec)/1000000.0);
    }
    status=0;
done:
    frontier_close();return status;
}
