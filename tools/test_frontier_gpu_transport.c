// Exercise the production transport against a separate, shared-memory device
// process: capability fallback, early submission, completion and failed export.
#include "../internal/frontiervideo/transport.c"
#include <assert.h>
#include <stdlib.h>
#include <sys/wait.h>

static void put(unsigned offset, uint32_t value) {
    *(volatile uint32_t *)(memory+offset)=value;
    barrier();
}

static void run_frame(uint32_t magic, uint32_t prior, int fail) {
    memset((void *)memory,0,MAP_SIZE);
    published=prior;
    put(0,prior);
    put(ACK+4,GPU_VIDEO_MAGIC);
    put(ACK,prior ^ 4u); // Old scanout sequence: not yet safe to export.
    put(GPU_MAILBOX+20,magic);
    uint8_t commands[64]={3,0,0,0},palette[8]={1,2,3,4,5,6,7,8};
    int deferred=magic==GPU_DEFER_MAGIC;
    pid_t child=fork();assert(child>=0);
    if(child==0) {
        if(!deferred) {
            // Legacy cores must see no request until ARM obtains scanout ACK.
            usleep(20000);
            assert(read_word(GPU_MAILBOX)==0);
            put(ACK,prior);
        }
        uint64_t deadline=millis()+1000;
        while(read_word(GPU_MAILBOX)!=1 && millis()<deadline)usleep(100);
        assert(read_word(GPU_MAILBOX)==1);
        assert(read_word(GPU_MAILBOX+4)==1);
        assert(read_word(GPU_MAILBOX+8)==(((prior&1u)^1u)|(deferred?2u:0u)));
        assert(read_word(GPU_MAILBOX+12)==(deferred?prior:0));
        assert(memcmp(gpu_memory,commands,sizeof(commands))==0);
        assert(memcmp(gpu_memory+0x200000,palette,sizeof(palette))==0);
        if(deferred)assert(read_word(ACK)==(prior^4u));
        // Submission is allowed early; publishing the new display sequence is
        // forbidden until the device reports successful, ownership-safe export.
        usleep(10000);
        assert(read_word(0)==prior);
        if(!fail)put(ACK,prior);
        put(GPU_MAILBOX+24,fail?5:0);
        put(GPU_MAILBOX+16,1);
        _exit(0);
    }
    int result=frontier_gpu_frame(commands,sizeof(commands),palette,sizeof(palette));
    int status;assert(waitpid(child,&status,0)==child);
    assert(WIFEXITED(status) && WEXITSTATUS(status)==0);
    assert(result==(fail?EIO:0));
    uint32_t next=((prior+4u)&~3u)|((prior&1u)^1u);
    assert(published==(fail?prior:next));
    assert(read_word(0)==published);
}

int main(void) {
    memory=mmap(NULL,MAP_SIZE,PROT_READ|PROT_WRITE,MAP_SHARED|MAP_ANONYMOUS,-1,0);
    gpu_memory=mmap(NULL,GPU_SIZE,PROT_READ|PROT_WRITE,MAP_SHARED|MAP_ANONYMOUS,-1,0);
    assert(memory!=MAP_FAILED && gpu_memory!=MAP_FAILED);
    run_frame(GPU_MAGIC,8,0);
    run_frame(GPU_GROUP_MAGIC,5,0);
    run_frame(GPU_DEFER_MAGIC,8,0);
    run_frame(GPU_DEFER_MAGIC,5,0);
    run_frame(GPU_DEFER_MAGIC,0xfffffffc,0);
    run_frame(GPU_DEFER_MAGIC,8,1);
    frontier_close();
    puts("PASS: IKG1/2 wait-before-submit, IKG3 overlap, both buffers, sequence wrap, failed export never published");
    return 0;
}
