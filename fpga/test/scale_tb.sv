`timescale 1ns/1ps
module scale_tb;
reg clk=0;
always #5 clk=~clk;
reg [11:0] w=1920,h=1080;
reg [2:0] mode=0;
wire [12:0] ax,ay;
video_scale_int dut(.CLK_VIDEO(clk),.HDMI_WIDTH(w),.HDMI_HEIGHT(h),.SCALE(mode),
 .hsize(12'd320),.vsize(12'd240),.arx_i(12'd4),.ary_i(12'd3),.arx_o(ax),.ary_o(ay));
task check;
 input [2:0] m;
 input integer width,height,expected_x,expected_y;
 begin
  mode=m; w=width; h=height;
  repeat(8000) @(negedge clk);
  if(ax!==expected_x || ay!==expected_y) $fatal(1,"scale %d @%dx%d got %d,%d expected %d,%d",m,width,height,ax,ay,expected_x,expected_y);
 end
endtask
initial begin
 // MiSTer hardware power-up initializes these unreset registers to zero.
 // Icarus needs the initial FSM state supplied for this unchanged vendor block.
 dut.scale_calc.cnt=0;
 dut.div.run=0;
 dut.mul.run=0;
 check(0,1920,1080,4,3);
 check(2,1920,1080,4096+1280,4096+960);
 check(2,1280,720,4096+960,4096+720);
 check(2,640,480,4096+640,4096+480);
 $display("PASS: MiSTer original aspect and HV integer scaling at 1080p, 720p, VGA 480p");
 $finish;
end
endmodule
