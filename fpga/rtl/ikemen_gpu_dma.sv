// Mailbox/command DMA for ikemen_gpu_raster. All DMA writes are bounded to the
// private RGBA snapshot, inactive scanout buffer selected by ARM, and status.
module ikemen_gpu_dma #(
    parameter GUARD_TIMEOUT_CYCLES=24609375 // 250 ms at 98.4375 MHz
) (
    input wire clk,reset,
    input wire ddr_busy,
    output wire [7:0] ddr_burst,
    output wire [28:0] ddr_addr,
    output wire ddr_rd,ddr_we,
    output wire [63:0] ddr_din,
    input wire [63:0] ddr_dout,
    input wire ddr_valid
);
    localparam POLL=0,POLL_WAIT=1,OPTIONS=2,OPTIONS_WAIT=3,
      FETCH=4,FETCH_WAIT=5,SUBMIT=6,RASTER=7,
      EXPORT_WAIT=8,EXPORT_CAPTURE=9,RGBA_WRITE=10,RGBA_WAIT=11,
      RGB_WRITE=12,RGB_WAIT=13,ADVANCE=14,RESULT_WRITE=15,RESULT_WAIT=16,
      ACK_WRITE=17,ACK_WAIT=18,BOOT_SUBMIT=19,BOOT_WAIT=20,
      EXPORT_GUARD=21,EXPORT_GUARD_WAIT=22,EXPORT_GUARD_PAUSE=23;
    localparam [28:0] MAILBOX=29'h0741c020; // 0x3a0e0100 / 8
    localparam [28:0] COMMANDS=29'h07000000; // 0x38000000 / 8
    localparam [28:0] SNAPSHOT=29'h07380000; // 0x39c00000 / 8
    localparam [28:0] VIDEO_ACK=29'h0741c000; // 0x3a0e0000 / 8
    reg [4:0] state;
    reg initialized;
    reg [31:0] frame_seq,count,command_index,error,cycles;
    reg deferred_export;
    reg [31:0] expected_video_ack;
    reg [24:0] guard_cycles;
    reg [10:0] idle_count;
    reg [511:0] command;
    reg [2:0] beat;
    reg [16:0] pixel;
    reg [28:0] output_base;
    reg [63:0] rgba_pair,rgb_quad;
    reg [7:0] burst;
    reg [28:0] address;
    reg read_req,write_req;
    reg [63:0] write_data;
    wire raster_ready,raster_fault,raster_read;
    wire [28:0] raster_address;
    wire [7:0] raster_burst;
    wire [31:0] peek;
    wire use_raster=state==RASTER || state==BOOT_WAIT;
    wire invalidate=state==OPTIONS_WAIT && ddr_valid;
    wire [511:0] boot_command={416'd0,32'd76800,32'd0,32'd3};
    wire [15:0] rgb565={peek[7:3],peek[15:10],peek[23:19]};
    assign ddr_burst=use_raster?raster_burst:burst;
    assign ddr_addr=use_raster?raster_address:address;
    assign ddr_rd=use_raster?raster_read:read_req;
    assign ddr_we=!use_raster && write_req;
    assign ddr_din=write_data;
    ikemen_gpu_raster raster (
        .clk(clk),.reset(reset),.invalidate(invalidate),
        .command_valid(state==SUBMIT || state==BOOT_SUBMIT),
        .command(state==BOOT_SUBMIT?boot_command:command),
        .ready(raster_ready),.fault(raster_fault),
        .mem_read(raster_read),.mem_address(raster_address),.mem_burst(raster_burst),
        .mem_busy(!use_raster || ddr_busy),.mem_data(ddr_dout),.mem_valid(use_raster && ddr_valid),
        .peek_address(pixel),.peek_data(peek)
    );
    always @(posedge clk) begin
        if(reset) begin
            state<=POLL;initialized<=0;frame_seq<=0;count<=0;command_index<=0;
            error<=0;cycles<=0;idle_count<=0;command<=0;beat<=0;pixel<=0;
            output_base<=0;rgba_pair<=0;rgb_quad<=0;burst<=1;address<=0;
            read_req<=0;write_req<=0;write_data<=0;
            deferred_export<=0;expected_video_ack<=0;guard_cycles<=0;
        end else begin
            if(!ddr_busy) begin read_req<=0;write_req<=0;end
            if(state!=POLL && state!=POLL_WAIT) cycles<=cycles+1'b1;
            if(state==EXPORT_GUARD || state==EXPORT_GUARD_WAIT || state==EXPORT_GUARD_PAUSE)
                if(guard_cycles<GUARD_TIMEOUT_CYCLES) guard_cycles<=guard_cycles+1'b1;
            case(state)
                POLL: if(idle_count!=0) idle_count<=idle_count-1'b1;
                      else if(!ddr_busy) begin address<=MAILBOX;burst<=1;read_req<=1;state<=POLL_WAIT;end
                POLL_WAIT: if(ddr_valid) begin
                    if(!initialized) begin frame_seq<=ddr_dout[31:0];state<=BOOT_SUBMIT;end
                    else if(ddr_dout[31:0]!=frame_seq) begin
                        frame_seq<=ddr_dout[31:0];count<=ddr_dout[63:32];command_index<=0;cycles<=0;error<=0;
                        guard_cycles<=0;
                        if(ddr_dout[63:32]>32768) begin error<=1;state<=RESULT_WRITE;end
                        else state<=OPTIONS;
                    end else begin state<=POLL;idle_count<=1023;end
                end
                BOOT_SUBMIT: if(raster_ready) state<=BOOT_WAIT;
                BOOT_WAIT: if(raster_ready) begin initialized<=1;state<=RESULT_WRITE;end
                OPTIONS: if(!ddr_busy) begin address<=MAILBOX+1'b1;burst<=1;read_req<=1;state<=OPTIONS_WAIT;end
                OPTIONS_WAIT: if(ddr_valid) begin
                    // Legacy options remain 0/1 with a zero upper word. IKG3
                    // bit 1 defers ownership checking until after rasterization.
                    // Its target must be opposite the expected displayed buffer.
                    if(ddr_dout[31:2]!=0 ||
                       (!ddr_dout[1] && ddr_dout[63:32]!=0) ||
                       (ddr_dout[1] && (ddr_dout[33] || ddr_dout[0]==ddr_dout[32]))) begin error<=2;state<=RESULT_WRITE;end
                    else begin
                        deferred_export<=ddr_dout[1];expected_video_ack<=ddr_dout[63:32];
                        output_base<=ddr_dout[0]?29'h07408008:29'h07400008;
                        if(count==0) begin pixel<=0;state<=ddr_dout[1]?EXPORT_GUARD:EXPORT_WAIT;end
                        else state<=FETCH;
                    end
                end
                FETCH: if(!ddr_busy) begin
                    address<=COMMANDS+(command_index<<3);burst<=8;read_req<=1;beat<=0;state<=FETCH_WAIT;
                end
                FETCH_WAIT: if(ddr_valid) begin
                    command[beat*64+:64]<=ddr_dout;
                    if(beat==7)state<=SUBMIT;else beat<=beat+1'b1;
                end
                SUBMIT: if(raster_ready) state<=RASTER;
                RASTER: if(raster_ready) begin
                    if(raster_fault) begin error<=3;state<=RESULT_WRITE;end
                    else if(command_index+1==count) begin pixel<=0;state<=deferred_export?EXPORT_GUARD:EXPORT_WAIT;end
                    else begin command_index<=command_index+1'b1;state<=FETCH;end
                end
                // Drawing only touches private block RAM. Export may not touch
                // the external buffers until scanout has latched the prior frame.
                // This publisher remains exclusive until the GPU completion ACK.
                EXPORT_GUARD: if(guard_cycles>=GUARD_TIMEOUT_CYCLES) begin error<=5;state<=RESULT_WRITE;end
                    else if(!ddr_busy) begin address<=VIDEO_ACK;burst<=1;read_req<=1;state<=EXPORT_GUARD_WAIT;end
                EXPORT_GUARD_WAIT: if(ddr_valid) begin
                    if(ddr_dout[63:32]==32'h494b5632 && ddr_dout[31:0]==expected_video_ack) state<=EXPORT_WAIT;
                    else begin idle_count<=1023;state<=EXPORT_GUARD_PAUSE;end
                end
                EXPORT_GUARD_PAUSE: if(idle_count==0) state<=EXPORT_GUARD;
                    else idle_count<=idle_count-1'b1;
                EXPORT_WAIT: state<=EXPORT_CAPTURE;
                EXPORT_CAPTURE: begin
                    rgba_pair[pixel[0]*32+:32]<=peek;
                    rgb_quad[pixel[1:0]*16+:16]<=rgb565;
                    state<=pixel[0]?RGBA_WRITE:ADVANCE;
                end
                RGBA_WRITE: if(!ddr_busy) begin
                    address<=SNAPSHOT+(pixel>>1);burst<=1;write_data<=rgba_pair;write_req<=1;state<=RGBA_WAIT;
                end
                RGBA_WAIT: if(!ddr_busy) state<=pixel[1:0]==3?RGB_WRITE:ADVANCE;
                RGB_WRITE: if(!ddr_busy) begin
                    address<=output_base+(pixel>>2);burst<=1;write_data<=rgb_quad;write_req<=1;state<=RGB_WAIT;
                end
                RGB_WAIT: if(!ddr_busy) state<=ADVANCE;
                ADVANCE: if(pixel==76799) state<=RESULT_WRITE;
                         else begin pixel<=pixel+1'b1;state<=EXPORT_WAIT;end
                RESULT_WRITE: if(!ddr_busy) begin
                    address<=MAILBOX+3;burst<=1;write_data<={cycles,error};write_req<=1;state<=RESULT_WAIT;
                end
                RESULT_WAIT: if(!ddr_busy) state<=ACK_WRITE;
                ACK_WRITE: if(!ddr_busy) begin
                    address<=MAILBOX+2;burst<=1;write_data<={32'h494b4733,frame_seq};write_req<=1;state<=ACK_WAIT;
                end
                ACK_WAIT: if(!ddr_busy) begin state<=POLL;idle_count<=1023;end
                default: begin state<=RESULT_WRITE;error<=4;end
            endcase
        end
    end
endmodule

