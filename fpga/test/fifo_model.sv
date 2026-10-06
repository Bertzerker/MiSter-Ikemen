// Test-only show-ahead FIFO. Does not model metastability or vendor timing.
module dcfifo #(
 parameter intended_device_family="", lpm_numwords=256, lpm_showahead="ON",
 lpm_type="dcfifo", lpm_width=64, lpm_widthu=8, overflow_checking="ON",
 rdsync_delaypipe=4, underflow_checking="ON", use_eab="ON", wrsync_delaypipe=4
)(input aclr, input [lpm_width-1:0] data, input rdclk,rdreq,wrclk,wrreq,
 output [lpm_width-1:0] q, output rdempty,wrfull,
 output [1:0] eccstatus, output rdfull,wrempty,
 output [lpm_widthu-1:0] rdusedw,wrusedw);
 reg [lpm_width-1:0] mem[0:lpm_numwords-1];
 integer wp=0,rp=0;
 assign q=mem[rp%lpm_numwords];
 assign rdempty=(wp==rp);
 assign wrfull=(wp-rp>=lpm_numwords);
 assign rdfull=wrfull, wrempty=rdempty, eccstatus=0;
 assign rdusedw=wp-rp, wrusedw=wp-rp;
 always @(posedge wrclk or posedge aclr)
  if(aclr) wp<=0;
  else if(wrreq) begin
   if(wrfull) $fatal(1,"FIFO overflow");
   mem[wp%lpm_numwords]<=data; wp<=wp+1;
  end
 always @(posedge rdclk or posedge aclr)
  if(aclr) rp<=0;
  else if(rdreq) begin
   if(rdempty) $fatal(1,"FIFO underflow");
   rp<=rp+1;
  end
endmodule
