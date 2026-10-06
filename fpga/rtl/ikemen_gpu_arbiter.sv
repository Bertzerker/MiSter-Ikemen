// Two single-command Avalon masters. Lock read ownership until the last beat;
// writes are single-beat. Round-robin idle grants also permit registered clients
// which wait for !busy before asserting their request.
module ikemen_gpu_arbiter (
    input wire clk,reset,
    input wire [7:0] a_burst,b_burst,
    input wire [28:0] a_addr,b_addr,
    input wire a_rd,a_we,b_rd,b_we,
    input wire [63:0] a_data,b_data,
    input wire [7:0] a_be,b_be,
    output wire a_busy,b_busy,a_valid,b_valid,
    output wire [7:0] burst,
    output wire [28:0] addr,
    output wire rd,we,
    output wire [63:0] data,
    output wire [7:0] be,
    input wire busy,valid
);
    reg grant=0,owner=0;
    reg [8:0] pending=0;
    assign a_busy=busy || pending!=0 || grant;
    assign b_busy=busy || pending!=0 || !grant;
    assign a_valid=valid && pending!=0 && !owner;
    assign b_valid=valid && pending!=0 && owner;
    assign burst=grant?b_burst:a_burst;
    assign addr=grant?b_addr:a_addr;
    assign data=grant?b_data:a_data;
    assign be=grant?b_be:a_be;
    assign rd=pending==0 && (grant?b_rd:a_rd);
    assign we=pending==0 && (grant?b_we:a_we);
    always @(posedge clk) begin
        if(reset) begin grant<=0;owner<=0;pending<=0;end
        else if(pending!=0) begin
            if(valid) begin pending<=pending-1'b1;if(pending==1)grant<=!owner;end
        end else if(!busy) begin
            if(rd) begin pending<={1'b0,burst};owner<=grant;end
            else grant<=!grant;
        end
    end
endmodule
