`timescale 1ns/1ps
module timing_tb;
reg clk=0, reset=1;
always #5 clk=~clk;
wire hs,vs,hb,vb,de,nf,nl;
wire [9:0] x;
wire [8:0] y;
openbor_video_timing dut(.clk(clk),.ce_pix(1'b1),.reset(reset),
 .h_adj(5'sd0),.v_adj(4'sd0),.hsync(hs),.vsync(vs),.hblank(hb),.vblank(vb),
 .de(de),.hcount(x),.vcount(y),.new_frame(nf),.new_line(nl));
integer active=0, sync_pixels=0, frame_pulses=0, line_pulses=0;
initial begin
 repeat(3) @(negedge clk);
 reset=0;
 repeat(420*262) begin
  @(negedge clk);
  if(de) active=active+1;
  if(!vs) sync_pixels=sync_pixels+1;
  if(nf) frame_pulses=frame_pulses+1;
  if(nl) line_pulses=line_pulses+1;
  if(de !== (x<320 && y<240)) $fatal(1,"active-area mismatch at %d,%d",x,y);
 end
 if(active!=76800 || sync_pixels!=1260 || frame_pulses!=1 || line_pulses!=262)
   $fatal(1,"timing counts %d %d %d %d",active,sync_pixels,frame_pulses,line_pulses);
 $display("PASS: 320x240 active pixels, 420x262 total, three-line VS, one vblank event");
 $finish;
end
endmodule
