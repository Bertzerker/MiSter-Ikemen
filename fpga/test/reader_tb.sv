`timescale 1ns/1ps
module reader_tb;
`ifdef GPU_ARBITRATION
localparam TEST_FRAMES=3;
`else
localparam TEST_FRAMES=35;
`endif
reg clk=0,vid=0,reset=1;
always #5 clk=~clk;
always #7 vid=~vid;
reg [2:0] divider=0;
always @(negedge vid) divider<=divider+1'b1;
wire ce=(divider==0);
wire de,hb,vb,nf,nl,hs,vs;
wire [9:0] x;
wire [8:0] y;
assign x=dut.tim_hcount, y=dut.tim_vcount, de=dut.tim_de;
assign hb=dut.tim_hblank, vb=dut.tim_vblank, nf=dut.tim_new_frame, nl=dut.tim_new_line;
reg busy=0,ready=0;
reg [63:0] dout=0;
wire [7:0] burst,be,r,g,b;
wire [28:0] addr;
wire [63:0] din;
wire rd,we,frame_ready,out_de;
wire video_busy,video_valid,video_rd,video_we;
wire [7:0] video_burst,video_be;
wire [28:0] video_addr;
wire [63:0] video_data;
`ifdef GPU_ARBITRATION
wire competitor_busy,competitor_valid;
integer competitor_beats=0;
always @(posedge clk) if(competitor_valid) competitor_beats<=competitor_beats+1;
ikemen_gpu_arbiter arb(.clk(clk),.reset(reset),
 .a_burst(video_burst),.a_addr(video_addr),.a_rd(video_rd),.a_we(video_we),.a_data(video_data),.a_be(video_be),
 .a_busy(video_busy),.a_valid(video_valid),
 .b_burst(8'd8),.b_addr(29'h07000000),.b_rd(!reset),.b_we(1'b0),.b_data(64'd0),.b_be(8'hff),
 .b_busy(competitor_busy),.b_valid(competitor_valid),
 .burst(burst),.addr(addr),.rd(rd),.we(we),.data(din),.be(be),.busy(busy),.valid(ready));
`else
assign video_busy=busy,video_valid=ready,burst=video_burst,addr=video_addr;
assign rd=video_rd,we=video_we,din=video_data,be=video_be;
`endif
openbor_video_top dut(.clk_sys(clk),.ddr_busy(video_busy),.ddr_burstcnt(video_burst),
 .ddr_addr(video_addr),.ddr_dout(dout),.ddr_dout_ready(video_valid),.ddr_rd(video_rd),.ddr_din(video_data),.ddr_be(video_be),.ddr_we(video_we),
 .clk_vid(vid),.ce_pix(ce),.reset(reset),.h_offset(3'd0),.v_offset(3'd0),
 .ioctl_download(1'b0),.ioctl_wr(1'b0),.ioctl_addr(27'd0),.ioctl_dout(8'd0),
 .joystick_0(32'd0),.joystick_1(32'd0),.joystick_2(32'd0),.joystick_3(32'd0),.joystick_l_analog_0(16'd0),
 .rs_play(1'b0),.rs_slot(3'd0),.vga_r(r),.vga_g(g),.vga_b(b),.vga_hs(hs),.vga_vs(vs),.vga_de(out_de),.clk_audio(clk),.enable(1'b1),.active(frame_ready));
integer cycles=0,left=0,index=0,acks=0,heartbeats=0,checked=0;
reg [28:0] read_addr;
reg [31:0] control=0,ack=0;
function [15:0] pixel(input integer p);
 pixel={5'(p%31+1),6'((p/320)%63+1),5'((p/320+p)%31+1)};
endfunction
always @(posedge clk) begin
 cycles<=cycles+1;
 // Exercise multi-cycle Avalon backpressure independently of returned data.
 busy<=cycles%17<5;
 ready<=0;
 if(!reset) begin
  if(we&&!busy) begin
   if(be!==8'hff || burst!==1) $fatal(1,"invalid metadata write");
   if(addr==29'h0741c001) begin
    if(din[63:32]!==32'h494b5631) $fatal(1,"heartbeat magic");
    heartbeats<=heartbeats+1;
   end
   if(addr==29'h0741c000) begin
    if(din[63:32]!==32'h494b5631 || din[31:0]!==control || !vb) $fatal(1,"ACK mismatch/outside vblank");
    ack<=din[31:0]; acks<=acks+1;
    if(acks==0) control<=5; // CPU publishes buffer 1 after baseline.
   end
  end
  if(rd&&!busy) begin
   if(left!=0) $fatal(1,"overlapping DDR requests");
   read_addr<=addr; left<=burst; index<=0;
   if(addr!=29'h07400000
`ifdef GPU_ARBITRATION
      && addr!=29'h07000000
`endif
      && (ack!=5 || addr<29'h07408008 || addr+burst>29'h0740cb08))
    $fatal(1,"video read outside acknowledged 240-line buffer: %h",addr);
  end
  else if(left>0 && cycles%7!=0) begin
   ready<=1;
   if(read_addr==29'h07400000) dout<={32'd0,control};
   else dout<={pixel((read_addr-29'h07408008+index)*4+3),pixel((read_addr-29'h07408008+index)*4+2),pixel((read_addr-29'h07408008+index)*4+1),pixel((read_addr-29'h07408008+index)*4)};
   left<=left-1; index<=index+1;
  end
 end
end
reg [15:0] expected;
// Loading/decoding can pause ARM publication for more than 30 refreshes.
// Once a complete picture exists it must remain visible, not time out to black.
always @(posedge vid) if(!reset && ce && de && checked>=320*240*2 && !frame_ready)
 $fatal(1,"last completed frame blanked while ARM publication was paused");
always @(posedge vid) if(!reset && ce && de && frame_ready) begin
 expected=pixel(y*320+x);
 #1;
 if(out_de!==1'b1) $fatal(1,"DE not aligned with RGB");
 if({r,g,b}!=={expected[15:11],expected[15:13],expected[10:5],expected[10:9],expected[4:0],expected[4:2]})
  $fatal(1,"pixel mismatch (%d,%d): %h %h %h expected RGB565 %h",x,y,r,g,b,expected);
 checked=checked+1;
 if(checked==320*240*TEST_FRAMES) begin
  if(acks!=2 || heartbeats<3) $fatal(1,"handshake counts");
`ifdef GPU_ARBITRATION
  if(competitor_beats<10000) $fatal(1,"competing GPU client made insufficient progress");
  $display("PASS: real video reader through GPU arbiter under continuous competing DDR reads (%0d beats)",competitor_beats);
`endif
  $display("PASS: vblank ACK, heartbeat, stalled DDR bursts, %0d full scanouts held without new ARM frames, no black frames or shifted pixels",TEST_FRAMES);
  $finish;
 end
end
initial begin
 repeat(20) @(negedge clk); reset=0;
 #600000000; $fatal(1,"simulation timeout; checked=%d state=%d",checked,dut.reader.state);
end
endmodule
