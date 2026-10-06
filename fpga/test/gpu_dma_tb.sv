`timescale 1ns/1ps
module gpu_dma_tb;
    reg clk=0;always #5 clk=~clk;
    reg reset=1;
    wire gb,gv,gr,gw;
    wire [7:0] gc;
    wire [28:0] ga;
    wire [63:0] gd;
    reg [63:0] dout=0;
    wire ab,av;
    reg ar=0;
    wire [7:0] burst;
    wire [28:0] addr;
    wire rd,we;
    wire [63:0] din;
    wire [7:0] be;
    reg busy=0,valid=0;
    ikemen_gpu_dma #(.GUARD_TIMEOUT_CYCLES(4096)) gpu(clk,reset,gb,gc,ga,gr,gw,gd,dout,gv);
    ikemen_gpu_arbiter arb(clk,reset,8'd80,gc,29'h07400008,ga,ar,1'b0,gr,gw,64'd0,gd,8'hff,8'hff,ab,gb,av,gv,burst,addr,rd,we,din,be,busy,valid);
    reg [511:0] commands [0:3];
    reg [31:0] frame_seq=0,count=0;
    reg [63:0] options=1;
    reg [31:0] video_ack=0,video_magic=32'h494b5632;
    integer guard_reads=0,test_case;
    integer cycle=0,pending=0,beat=0,delay_count=0;
    integer snapshots=0,pixels=0,acks=0,video_beats=0,video_pending=0;
    reg [31:0] address;
    reg [31:0] status_error=0,ack_sequence=32'hffffffff;
    integer offset,p;
    reg [63:0] want;
    function automatic [63:0] memory_read(input [31:0] a);
        integer ci,word_index;
        begin
            if(a==32'h3a0e0100)memory_read={count,frame_seq};
            else if(a==32'h3a0e0108)memory_read=options;
            else if(a==32'h3a0e0000)memory_read={video_magic,video_ack};
            else if(a>=32'h38000000 && a<32'h38000100) begin
                ci=(a-32'h38000000)/64;word_index=((a-32'h38000000)%64)/8;
                memory_read=commands[ci][word_index*64+:64];
            end else if(a>=32'h38200000 && a<32'h38200800)memory_read=64'h0000ff00ff00ff00;
            else if(a>=32'h38400000 && a<32'h38400040)memory_read=0;
            else if(a>=32'h38400100 && a<32'h38400140)memory_read=64'hffff0000ffff0000;
            else if(a>=32'h3a000040 && a<32'h3a0002c0)memory_read=64'h123456789abcdef0;
            else begin $fatal(1,"unexpected GPU DMA read %h",a);memory_read=0;end
        end
    endfunction
    always @(posedge clk) begin
        cycle<=cycle+1;
        busy<=pending!=0 || cycle%13==0;
        valid<=0;
        if(!reset) begin
            if(!ab) begin
                ar<=0;
                if(!ar && video_pending==0 && cycle%31==0)ar<=1;
                if(ar)video_pending<=80;
            end
            if(av) begin
                if(video_pending==0 || dout!==64'h123456789abcdef0) $fatal(1,"scanout response routed incorrectly");
                video_pending<=video_pending-1;video_beats<=video_beats+1;
            end
            if(rd && !busy) begin
                if(pending!=0) $fatal(1,"overlapping DDR burst");
                address<=addr<<3;pending<=burst;beat<=0;delay_count<=2;
                if((addr<<3)==32'h3a0e0000)guard_reads<=guard_reads+1;
            end
            if(pending!=0) begin
                if(delay_count>0)delay_count<=delay_count-1;
                else if(cycle%7!=0) begin
                    dout<=memory_read(address+beat*8);valid<=1;
                    pending<=pending-1;beat<=beat+1;
                end
            end
            if(we && !busy) begin
                if(burst!=1 || be!=255)$fatal(1,"invalid write burst/mask");
                if((addr<<3)==32'h3a0e0110)begin
                    if(din[63:32]!=32'h494b4733)$fatal(1,"GPU magic");
                    ack_sequence<=din[31:0];acks<=acks+1;
                end else if((addr<<3)==32'h3a0e0118)status_error<=din[31:0];
                else if((addr<<3)>=32'h39c00000 && (addr<<3)<32'h39c4b000)begin
                    if(options[1] && (video_magic!=32'h494b5632 || video_ack!=options[63:32]))$fatal(1,"export before scanout ownership");
                    p=((addr<<3)-32'h39c00000)/4;
                    want=p<1280?64'hff0000ffff0000ff:p<2560?64'hff00ff00ff00ff00:p<3840?64'hffff0000ffff0000:64'd0;
                    if(din!==want)$fatal(1,"RGBA snapshot %d got %h want %h",p,din,want);
                    snapshots<=snapshots+1;
                end else if((addr<<3)>=32'h3a040040 && (addr<<3)<32'h3a065840)begin
                    p=((addr<<3)-32'h3a040040)/2;
                    want=p<1280?64'hf800f800f800f800:p<2560?64'h07e007e007e007e0:p<3840?64'h001f001f001f001f:64'd0;
                    if(din!==want)$fatal(1,"RGB565 pixel %d got %h want %h",p,din,want);
                    pixels<=pixels+4;
                end else $fatal(1,"DMA write escaped allowed buffers: %h",addr<<3);
            end
        end
        if(cycle>4000000)$fatal(1,"DMA timeout");
    end
    initial begin
        commands[0]=0;commands[0][1:0]=3;commands[0][64+:32]=76800;
        commands[1]=0;commands[1][10:8]=7;commands[1][5:4]=3;commands[1][64+:32]=320;
        commands[1][352+:64]=64'h000000ffff0000ff;
        commands[2]=0;commands[2][1:0]=1;commands[2][10:8]=3;commands[2][5:4]=3;
        commands[2][32+:32]=1280;commands[2][64+:32]=320;
        commands[2][96+:32]=32'h38400000;commands[2][128+:32]=1;commands[2][160+:32]=1;
        commands[2][320+:32]=32'h38200000;
        commands[3]=commands[2];commands[3][1:0]=2;
        commands[3][32+:32]=2560;commands[3][96+:32]=32'h38400100;
        repeat(4)@(negedge clk);reset=0;
        wait(ack_sequence==0);
        @(negedge clk);count=4;frame_seq=1;
        wait(ack_sequence==1);
        if(status_error!=0 || pixels!=76800 || snapshots!=38400 || video_beats<80)$fatal(1,"incomplete frame or video starvation");
        @(negedge clk);count=32769;frame_seq=2;
        wait(ack_sequence==2);
        if(status_error!=1 || pixels!=76800)$fatal(1,"invalid command count accepted");
        // Rasterization must finish with the prior frame still unacknowledged,
        // but not one external pixel may be exported until the exact ACK and
        // live scanout magic match. Exercise a sequence near wraparound too.
        @(negedge clk);count=4;options={32'hfffffffc,32'd3};frame_seq=3;
        wait(gpu.state==21);
        if(gpu.command_index!=3 || pixels!=76800 || snapshots!=38400)$fatal(1,"drawing did not overlap scanout wait");
        wait(guard_reads>=1);
        @(negedge clk);video_ack=32'hfffffffc;video_magic=0;
        repeat(1200)@(negedge clk);
        if(pixels!=76800 || snapshots!=38400 || ack_sequence!=2)$fatal(1,"invalid magic allowed export");
        video_magic=32'h494b5632;video_ack=32'hfffffff8;
        repeat(1200)@(negedge clk);
        if(pixels!=76800 || snapshots!=38400 || ack_sequence!=2)$fatal(1,"wrong sequence allowed export");
        video_ack=32'hfffffffc;
        wait(ack_sequence==3);
        if(status_error!=0 || pixels!=153600 || snapshots!=76800)$fatal(1,"deferred frame incomplete");
        // Missing ACK: return a bounded error without exporting anything.
        @(negedge clk);count=0;video_ack=0;frame_seq=4;
        wait(ack_sequence==4);
        if(status_error!=5 || pixels!=153600 || snapshots!=76800)$fatal(1,"ownership timeout failed");
        // Reject writing the expected displayed buffer, reserved sequence bit,
        // unknown flags, or a nonzero legacy reserved word.
        for(test_case=0;test_case<4;test_case=test_case+1)begin
            @(negedge clk);
            case(test_case)
                0:options={32'd1,32'd3};
                1:options={32'd2,32'd3};
                2:options=5;
                3:options={32'd4,32'd1};
            endcase
            frame_seq=5+test_case;
            wait(ack_sequence==frame_seq);
            if(status_error!=2 || pixels!=153600 || snapshots!=76800)$fatal(1,"invalid options accepted");
        end
        $display("PASS: GPU pixels/DMA/arbitration, legacy/deferred export, exact ownership ACK, timeout and invalid options");
        $finish;
    end
endmodule

