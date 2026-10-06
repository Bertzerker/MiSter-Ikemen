// Ordered 2D row rasterizer. A 4 KiB texture cache uses 64-byte lines; framebuffer
// blending stays in FPGA block RAM. Commands are consumed only while ready.
// Copyright (C) 2026. SPDX-License-Identifier: GPL-3.0-or-later
module ikemen_gpu_raster (
    input wire clk, reset,
    input wire invalidate, // new frame: palette/texture arena may have changed
    input wire command_valid,
    input wire [511:0] command,
    output wire ready,
    output reg fault,
    output reg mem_read,
    output reg [28:0] mem_address,
    output wire [7:0] mem_burst,
    input wire mem_busy,
    input wire [63:0] mem_data,
    input wire mem_valid,
    input wire [16:0] peek_address,
    output wire [31:0] peek_data
);
    localparam IDLE=0, CHECK=1, PAL_REQUEST=2, PAL_WAIT=3,
      SAMPLE=4, TEX_REQUEST=5, TEX_WAIT=6, FB_WAIT=7, BLEND=8, CLEAR=9,
      PAL_SAMPLE=10,TEX_MUL=11,TEX_INDEX=12,TEX_ADDRESS=13,
      TEX_LOOKUP=14,TEX_SAMPLE=15,VALIDATE=16,
      BLEND_START=17,BLEND_WAIT1=18,BLEND_WAIT2=19,TEX_READ=20,FAST_WRITE=21,NEXT_ROW=22;
    reg [4:0] state;
    reg [31:0] fb [0:76799];
    reg [63:0] palette [0:255];
    reg [63:0] palette_q;
    reg [63:0] texture_cache [0:511];
    reg [63:0] cache_q;
    reg [19:0] cache_tag [0:63];
    reg [63:0] cache_valid;
    reg [31:0] fb_q;
    reg [31:0] fast_pixel;
    reg [15:0] sx_q,sy_q;
    reg [31:0] row_product,texel_q,byte_address,texture_size;
    reg row_product_valid;
    reg [63:0] tex_word;
    reg [16:0] destination;
    reg [31:0] remaining, row_length;
    reg [16:0] row_start;
    reg [1:0] rows_left;
    reg signed [31:0] start_u;
    reg [95:0] row_vs;
    reg [1:0] kind;
    reg [2:0] mode;
    reg [31:0] texture_base, palette_base, loaded_palette;
    reg [15:0] texture_width, texture_height;
    reg signed [31:0] u, v, du, dv;
    reg [63:0] source, flat_source;
    reg palette_valid;
    reg [7:0] pal_index;
    reg [2:0] beat;
    wire [31:0] blended;
    // Alpha is part of the framebuffer too. Only bypass a blend when all
    // four resulting channels are provably identical to the general path.
    wire source_noop = source[31:24]==0 &&
      (mode==4 || ((mode==1 || mode==3 || mode==6) && source[23:0]==0) ||
                  ((mode==0 || mode==2 || mode==5) && source[55:32]==0));
    wire source_replace = mode==7 || (source[31:24]==255 && (mode==2 || mode==3 || mode==4));
    wire signed [31:0] sample_x = u >>> 16;
    wire signed [31:0] sample_y = v >>> 16;
    wire [15:0] sx = sample_x < 0 ? 16'd0 : sample_x >= $signed({1'b0,texture_width}) ? texture_width-16'd1 : sample_x[15:0];
    wire [15:0] sy = sample_y < 0 ? 16'd0 : sample_y >= $signed({1'b0,texture_height}) ? texture_height-16'd1 : sample_y[15:0];
    wire [32:0] texture_end = {1'b0,texture_base} + (kind == 2 ? ({1'b0,texture_size} << 2) : {1'b0,texture_size});
    wire [31:0] next_byte_address = texture_base+(kind==2?(texel_q<<2):texel_q);
    // Start the synchronous read while registering the byte address. Hits can
    // consume that word in LOOKUP; refills retain a read-after-write bubble.
    wire [8:0] cache_read_address = state==TEX_ADDRESS ? next_byte_address[11:3] : byte_address[11:3];
    wire [63:0] cached_sample = cache_q >> {byte_address[2:0],3'b000};
    // On a hit the index is already available. Read the palette in the same
    // cycle as extracting it, rather than waiting for the tex_word register.
    wire [7:0] palette_read_index = state==TEX_LOOKUP ? cached_sample[7:0] : tex_word[7:0];
    assign mem_burst = 8;
    assign ready = state == IDLE;
    // Export and rasterization never overlap. Share one synchronous read port
    // so Quartus can implement the framebuffer as simple dual-port block RAM.
    wire [16:0] fb_read_address = ready ? peek_address : destination;
    assign peek_data = fb_q;
    // The synchronous framebuffer output is stable by FB_WAIT for every kind.
    // Feed the pipeline directly, avoiding a redundant destination register.
    ikemen_gpu_blend_pipe blender(clk, mode, source, fb_q, blended);
    function automatic [7:0] mul255(input [7:0] a, input [7:0] b);
        reg [15:0] t;
        reg [8:0] low_sum;
        // a*b+128 <= 65153. Only the low-byte carry affects the high
        // byte of t+(t>>8); avoid the wider second carry chain.
        begin
            t=a*b+16'd128;
            low_sum={1'b0,t[7:0]}+{1'b0,t[15:8]};
            mul255=t[15:8]+low_sum[8];
        end
    endfunction
    always @(posedge clk) begin
        // Synchronous block RAM ports, no reset of the memory array itself.
        fb_q <= fb[fb_read_address];
        palette_q <= palette[palette_read_index];
        cache_q <= texture_cache[cache_read_address];
        if(!reset && state==TEX_WAIT && mem_valid)
            texture_cache[{byte_address[11:6],beat}] <= mem_data;
        if(!reset && (state == BLEND || state == CLEAR || state == FAST_WRITE))
            fb[destination] <= state == CLEAR ? 32'd0 : state == FAST_WRITE ? fast_pixel : blended;
        if(reset) begin
            state<=IDLE; fault<=0; mem_read<=0; mem_address<=0;
            palette_valid<=0; cache_valid<=0; destination<=0;
            row_length<=0;row_start<=0;rows_left<=0;start_u<=0;row_vs<=0;
            remaining<=0; kind<=0; mode<=0; texture_base<=0;
            palette_base<=0; loaded_palette<=0; texture_width<=1;
            texture_height<=1; u<=0;v<=0;du<=0;dv<=0;
            source<=0;flat_source<=0;pal_index<=0;beat<=0;
            sx_q<=0;sy_q<=0;row_product<=0;texel_q<=0;byte_address<=0;
            row_product_valid<=0;
            texture_size<=0;tex_word<=0;fast_pixel<=0;
        end else begin
            if(!mem_busy) mem_read<=0;
            if(invalidate && ready) begin palette_valid<=0;cache_valid<=0;fault<=0;end
            case(state)
                IDLE: if(command_valid) begin
                    row_product_valid<=0;
                    // word0: kind (0 flat, 1 indexed, 2 RGBA, 3 clear), mode[10:8]
                    kind<=command[1:0]; mode<=command[10:8];
                    rows_left<=command[5:4];row_start<=command[32 +: 17];
                    row_length<=command[64 +: 32];start_u<=command[192 +: 32];
                    row_vs<=command[416 +: 96];
                    destination<=command[32 +: 17];
                    remaining<=command[64 +: 32];
                    texture_base<=command[96 +: 32];
                    texture_width<=command[128 +: 16];
                    texture_height<=command[160 +: 16];
                    u<=command[192 +: 32];v<=command[224 +: 32];
                    du<=command[256 +: 32];dv<=command[288 +: 32];
                    palette_base<=command[320 +: 32];
                    flat_source<=command[352 +: 64];
                    if(command[32 +: 32]>=76800 || command[64 +: 32]>76800-command[32 +: 32] || command[31:11]!=0 || command[7:6]!=0 || command[3:2]!=0 || (command[5:4]!=0 && (command[1:0]==3 || command[64 +: 32]>320 || {1'b0,command[32 +: 32]}+command[5:4]*33'd320+{1'b0,command[64 +: 32]}>76800))) begin
                        fault<=1;
                    end else state<=CHECK;
                end
                CHECK: begin texture_size<=texture_width*texture_height;state<=VALIDATE;end
                VALIDATE: begin
                    if(remaining==0) state<=IDLE;
                    else if(kind==3) state<=CLEAR;
                    else if(kind==0) state<=SAMPLE;
                    else if(texture_width==0 || texture_height==0 || texture_width>8192 || texture_height>8192 || texture_base<32'h38400000 || texture_end>33'h039c00000 || (kind==2 && texture_base[1:0]!=0)) begin fault<=1;state<=IDLE;end
                    else if(kind==1 && (palette_base<32'h38200000 || palette_base>32'h383ff800 || palette_base[5:0]!=0)) begin fault<=1;state<=IDLE;end
                    else if(kind==1 && (!palette_valid || loaded_palette!=palette_base)) begin pal_index<=0;state<=PAL_REQUEST;end
                    else state<=SAMPLE;
                end
                PAL_REQUEST: if(!mem_busy) begin
                    mem_address<=(palette_base>>3)+pal_index;
                    mem_read<=1;beat<=0;state<=PAL_WAIT;
                end
                PAL_WAIT: if(mem_valid) begin
                    palette[pal_index+beat]<=mem_data;
                    if(beat==7) begin
                        if(pal_index==248) begin loaded_palette<=palette_base;palette_valid<=1;state<=SAMPLE;end
                        else begin pal_index<=pal_index+8;state<=PAL_REQUEST;end
                    end else beat<=beat+1'b1;
                end
                SAMPLE: begin
                    if(kind==0) begin source<=flat_source;state<=FB_WAIT;end
                    else if(row_product_valid) begin texel_q<=row_product+sx;state<=TEX_ADDRESS;end
                    else begin sx_q<=sx;sy_q<=sy;state<=TEX_MUL;end
                end
                // Separate clamping, address arithmetic, cache selection and
                // color conversion. The former combinational chain exceeded
                // the 10.158 ns system-clock period by over 14 ns after fitting.
                TEX_MUL: begin
                    row_product<=sy_q*texture_width;
                    // V, including signed wrap and clamping, is constant only
                    // for dv==0. Grouped rows each restart this calculation.
                    row_product_valid<=dv==0;state<=TEX_INDEX;
                end
                TEX_INDEX: begin texel_q<=row_product+sx_q;state<=TEX_ADDRESS;end
                TEX_ADDRESS: begin
                    byte_address<=next_byte_address;
                    state<=TEX_LOOKUP;
                end
                TEX_LOOKUP: begin
                    if(!cache_valid[byte_address[11:6]] || cache_tag[byte_address[11:6]]!=byte_address[31:12]) state<=TEX_REQUEST;
                    else begin
                        tex_word<=cached_sample;
                        state<=kind==1?PAL_SAMPLE:TEX_SAMPLE;
                    end
                end
                TEX_READ: begin
                    // The final refill beat can be the requested word. Allow
                    // a full synchronous read after that write before LOOKUP.
                    state<=TEX_LOOKUP;
                end
                TEX_SAMPLE: begin
                        if(kind==1) state<=PAL_SAMPLE;
                        else source<={8'd0,tex_word[23:0],tex_word[31:24],mul255(tex_word[23:16],tex_word[31:24]),mul255(tex_word[15:8],tex_word[31:24]),mul255(tex_word[7:0],tex_word[31:24])};
                        if(kind!=1) state<=FB_WAIT;
                end
                TEX_REQUEST: if(!mem_busy) begin
                    mem_address<={byte_address[31:6],3'b000};
                    mem_read<=1;beat<=0;state<=TEX_WAIT;
                end
                TEX_WAIT: if(mem_valid) begin
                    if(beat==7) begin
                        cache_tag[byte_address[11:6]]<=byte_address[31:12];
                        cache_valid[byte_address[11:6]]<=1;
                        state<=TEX_READ;
                    end
                    else beat<=beat+1'b1;
                end
                FB_WAIT: begin
                    if(source_noop) begin
                        destination<=destination+1'b1;remaining<=remaining-1'b1;
                        u<=u+du;v<=v+dv;
                        state<=remaining==1?(rows_left==0?IDLE:NEXT_ROW):SAMPLE;
                    end else if(source_replace) begin
                        fast_pixel<=mode==4?32'd0:mode==3?source[31:0]:{source[31:24],source[55:32]};
                        state<=FAST_WRITE;
                    end else state<=BLEND_WAIT1;
                end
                BLEND_START: state<=BLEND_WAIT1;
                BLEND_WAIT1: state<=BLEND_WAIT2;
                BLEND_WAIT2: state<=BLEND;
                PAL_SAMPLE: begin source<=palette_q;state<=FB_WAIT;end
                BLEND,FAST_WRITE: begin
                    destination<=destination+1'b1;remaining<=remaining-1'b1;
                    u<=u+du;v<=v+dv;
                    state<=remaining==1?(rows_left==0?IDLE:NEXT_ROW):SAMPLE;
                end
                NEXT_ROW: begin
                    row_product_valid<=0;
                    row_start<=row_start+17'd320;destination<=row_start+17'd320;
                    remaining<=row_length;u<=start_u;v<=row_vs[31:0];
                    row_vs<=row_vs>>32;rows_left<=rows_left-1'b1;state<=SAMPLE;
                end
                CLEAR: begin
                    destination<=destination+1'b1;remaining<=remaining-1'b1;
                    if(remaining==1) state<=IDLE;
                end
                default: begin state<=IDLE;fault<=1;end
            endcase
        end
    end
endmodule
