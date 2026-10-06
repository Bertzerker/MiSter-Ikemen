`timescale 1ns/1ps
module gpu_blend_tb;
    reg clk=0;always #5 clk=~clk;
    reg [2:0] mode;
    reg [63:0] source;
    reg [31:0] destination;
    wire [31:0] result;
    ikemen_gpu_blend dut(.*);
    wire [31:0] piped;
    ikemen_gpu_blend_pipe pipeline(clk,mode,source,destination,piped);
    integer a,d,m,c,n,s,p,want,got;
    task check;
        begin
            repeat(3)@(posedge clk);#1;
            for(c=0;c<4;c=c+1) begin
                a=source[31:24]; d=destination[c*8 +: 8];
                s=c==3?a:source[32+c*8 +: 8];
                p=c==3?(a*a+127)/255:source[c*8 +: 8];
                case(mode)
                    0: begin want=d+s; if(want>255)want=255; end
                    1: begin want=d+p; if(want>255)want=255; end
                    2: want=(s+d-(d*a+127)/255)&255;
                    3: want=(p+d-(d*a+127)/255)&255;
                    4: want=d-(d*a+127)/255;
                    5: begin want=d-s; if(want<0)want=0; end
                    6: begin want=d-p; if(want<0)want=0; end
                    7: want=s;
                endcase
                got=result[c*8 +: 8];
                if(got!=want) $fatal(1,"blend mode %d channel %d: got %d want %d src %h dst %h",mode,c,got,want,source,destination);
                if(piped[c*8+:8]!=want) $fatal(1,"pipelined blend mode %d channel %d: got %d want %d",mode,c,piped[c*8+:8],want);
            end
        end
    endtask
    initial begin
        for(m=0;m<8;m=m+1) begin
            mode=m;
            for(n=0;n<65536;n=n+1) begin
                source={8'd0,8'd255,8'd128,8'd0,n[15:8],n[7:0],8'd0,8'd255};
                destination={n[7:0],n[15:8],n[7:0],n[15:8]};
                check();
            end
            for(n=0;n<1000;n=n+1) begin
                source={$random,$random}; destination=$random; check();
            end
        end
        $display("PASS: combinational and pipelined hardware blend, all eight modes, exhaustive alpha/destination pairs and randomized sources");
        $finish;
    end
endmodule
