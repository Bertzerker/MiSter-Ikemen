// IKEMEN's integer blend rules. Source layout matches buildPalTable:
// [31:0] = premultiplied ABGR, [63:32] = unused/B/G/R raw channels.
// Copyright (C) 2026. SPDX-License-Identifier: GPL-3.0-or-later
module ikemen_gpu_blend (
    input wire [2:0] mode,
    input wire [63:0] source,
    input wire [31:0] destination,
    output reg [31:0] result
);
    function automatic [7:0] mul255(input [7:0] a, input [7:0] b);
        reg [16:0] t;
        begin
            t = a * b + 17'd128;
            mul255 = (t + (t >> 8)) >> 8;
        end
    endfunction
    function automatic [7:0] sat(input signed [10:0] n);
        begin sat = n < 0 ? 8'd0 : n > 255 ? 8'd255 : n[7:0]; end
    endfunction
    integer c;
    reg [7:0] raw, premul, dst, inv;
    reg [7:0] alpha;
    always @* begin
        alpha = source[31:24];
        result = 0;
        raw = 0; premul = 0; dst = 0; inv = 0;
        for (c = 0; c < 4; c = c + 1) begin
            raw = c == 3 ? alpha : source[32+c*8 +: 8];
            premul = c == 3 ? mul255(alpha, alpha) : source[c*8 +: 8];
            dst = destination[c*8 +: 8];
            inv = dst - mul255(dst, alpha);
            case (mode)
                0: result[c*8 +: 8] = sat($signed({3'b0,dst}) + $signed({3'b0,raw}));
                1: result[c*8 +: 8] = sat($signed({3'b0,dst}) + $signed({3'b0,premul}));
                2: result[c*8 +: 8] = raw + inv; // byte wrap matches software
                3: result[c*8 +: 8] = premul + inv;
                4: result[c*8 +: 8] = inv;
                5: result[c*8 +: 8] = sat($signed({3'b0,dst}) - $signed({3'b0,raw}));
                6: result[c*8 +: 8] = sat($signed({3'b0,dst}) - $signed({3'b0,premul}));
                7: result[c*8 +: 8] = raw;
            endcase
        end
    end
endmodule

// Three-clock implementation used by the rasterizer. Keep the DDR/BRAM read,
// multiply, rounding and final add/subtract off a single combinational path.
module ikemen_gpu_blend_pipe (
    input wire clk,
    input wire [2:0] mode,
    input wire [63:0] source,
    input wire [31:0] destination,
    output reg [31:0] result
);
    reg [2:0] mode1,mode2;
    reg [7:0] dst1[0:3],dst2[0:3],raw1[0:3],raw2[0:3];
    reg [7:0] premul1[0:3],premul2[0:3],inverse2[0:3];
    reg [15:0] product1[0:3],alpha_product1;
    function automatic [7:0] rounded(input [15:0] product);
        reg [16:0] t;
        begin t={1'b0,product}+17'd128;rounded=(t+(t>>8))>>8;end
    endfunction
    function automatic [7:0] sat(input signed [10:0] n);
        begin sat=n<0?8'd0:n>255?8'd255:n[7:0];end
    endfunction
    integer c;
    always @(posedge clk) begin
        mode1<=mode;mode2<=mode1;
        alpha_product1<=source[31:24]*source[31:24];
        for(c=0;c<4;c=c+1) begin
            dst1[c]<=destination[c*8+:8];dst2[c]<=dst1[c];
            raw1[c]<=c==3?source[31:24]:source[32+c*8+:8];raw2[c]<=raw1[c];
            premul1[c]<=source[c*8+:8];
            premul2[c]<=c==3?rounded(alpha_product1):premul1[c];
            product1[c]<=destination[c*8+:8]*source[31:24];
            inverse2[c]<=dst1[c]-rounded(product1[c]);
            case(mode2)
                0: result[c*8+:8]<=sat($signed({3'b0,dst2[c]})+$signed({3'b0,raw2[c]}));
                1: result[c*8+:8]<=sat($signed({3'b0,dst2[c]})+$signed({3'b0,premul2[c]}));
                2: result[c*8+:8]<=raw2[c]+inverse2[c];
                3: result[c*8+:8]<=premul2[c]+inverse2[c];
                4: result[c*8+:8]<=inverse2[c];
                5: result[c*8+:8]<=sat($signed({3'b0,dst2[c]})-$signed({3'b0,raw2[c]}));
                6: result[c*8+:8]<=sat($signed({3'b0,dst2[c]})-$signed({3'b0,premul2[c]}));
                7: result[c*8+:8]<=raw2[c];
            endcase
        end
    end
endmodule
