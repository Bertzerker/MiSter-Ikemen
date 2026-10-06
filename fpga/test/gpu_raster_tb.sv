`timescale 1ns/1ps
module gpu_raster_tb;
    reg clk=0; always #5 clk=~clk;
    reg reset=1,invalidate=0,command_valid=0;
    reg [511:0] command=0;
    wire ready,fault,mem_read;
    wire [28:0] mem_address;
    wire [7:0] mem_burst;
    reg mem_busy=0,mem_valid=0;
    reg [63:0] mem_data=0;
    reg [16:0] peek_address=0;
    wire [31:0] peek_data;
    ikemen_gpu_raster dut(.*);
    reg [31:0] expected [0:76799];
    integer cycle=0,pending=0,delay_count=0,beat=0;
    reg [31:0] address;
    integer i,k,n,x,index,a,r,g,b;
    reg [7:0] alpha;
    reg [63:0] value;
    reg [7:0] texture_xor=0;
    integer texture_reads=0,reads_before;
    reg [2:0] reference_mode;
    reg [63:0] reference_source;
    reg [31:0] reference_destination;
    wire [31:0] reference_result;
    ikemen_gpu_blend reference_blend(reference_mode,reference_source,reference_destination,reference_result);
    integer blend_mode,variant,cycles_before;
    integer grouped_kind,grouped_mode,grouped_rows,j;
    reg [511:0] grouped_command;
    reg signed [31:0] test_u,test_v,test_du,test_dv;
    integer test_x,test_y,sampling_kind,sampling_case;
    function automatic [63:0] read_memory(input [31:0] addr);
        integer j,idx;
        reg [63:0] data;
        reg [7:0] rr,gg,bb,aa;
        begin
            data=0;
            if(addr>=32'h38200000 && addr<32'h38200800) begin
                idx=(addr-32'h38200000)/8;
                rr=idx;gg=255-idx;bb=idx^85;aa=idx==0?0:255;
                data={8'd0,bb,gg,rr,aa,aa==0?8'd0:bb,aa==0?8'd0:gg,aa==0?8'd0:rr};
            end else if(addr>=32'h38400000 && addr<32'h39c00000) begin
                for(j=0;j<8;j=j+1) data[j*8 +: 8]=((addr+j)&255)^texture_xor;
            end else $fatal(1,"unbounded texture/palette read %h",addr);
            read_memory=data;
        end
    endfunction
    always @(posedge clk) begin
        cycle<=cycle+1;
        mem_busy<=((cycle%11)==2 || (cycle%11)==3 || pending!=0);
        mem_valid<=0;
        if(mem_read && !mem_busy) begin
            if(pending!=0 || mem_burst!=8) $fatal(1,"bad burst ownership");
            address<=mem_address<<3;pending<=8;delay_count<=3;beat<=0;
            if((mem_address<<3)>=32'h38400000) texture_reads<=texture_reads+1;
        end
        if(pending!=0) begin
            if(delay_count!=0) delay_count<=delay_count-1;
            else if(cycle%5!=0) begin
                mem_data<=read_memory(address+beat*8);mem_valid<=1;
                pending<=pending-1;beat<=beat+1;
            end
        end
        if(cycle>1000000) $fatal(1,"raster timeout");
    end
    task submit;
        begin
            wait(ready);@(negedge clk);command_valid=1;
            @(negedge clk);command_valid=0;
            wait(ready);@(negedge clk);
            if(fault) $fatal(1,"unexpected command fault");
        end
    endtask
    task compare(input integer start,input integer length);
        integer j;
        begin
            for(j=start;j<start+length;j=j+1) begin
                @(negedge clk);peek_address=j;
                @(posedge clk);#1;
                if(peek_data!==expected[j]) $fatal(1,"pixel %d got %h expected %h",j,peek_data,expected[j]);
            end
        end
    endtask
    initial begin
        // Exhaustively verify the shortened RGBA premultiplication rounding.
        for(i=0;i<256;i=i+1) for(k=0;k<256;k=k+1) begin
            n=i*k+128;
            if(dut.mul255(i,k)!==((n+(n>>8))>>8))
                $fatal(1,"RGBA multiply mismatch color=%d alpha=%d",i,k);
        end
        repeat(4) @(negedge clk);reset=0;
        command=0;command[1:0]=3;command[64+:32]=76800;submit();
        for(i=0;i<76800;i=i+1) expected[i]=0;
        compare(0,320);compare(76480,320);
        // Flat replacement establishes a destination with nonzero alpha.
        command=0;command[10:8]=7;command[32+:32]=100;command[64+:32]=640;
        command[352+:64]=64'h007f402080402010;submit();
        for(i=100;i<740;i=i+1) expected[i]=32'h807f4020;
        compare(100,640);
        // Indexed nearest-neighbor: clipped negative U, palette mask, right clamp.
        command=0;command[1:0]=1;command[10:8]=3;
        command[32+:32]=100;command[64+:32]=320;
        command[96+:32]=32'h38400000;command[128+:32]=128;command[160+:32]=3;
        command[192+:32]=-32'sd655360;command[256+:32]=65536;
        command[320+:32]=32'h38200000;submit();
        for(i=0;i<320;i=i+1) begin
            x=i-10;if(x<0)x=0;if(x>127)x=127;
            value=read_memory(32'h38200000+x*8);
            if(x!=0)expected[100+i]=value[31:0];
        end
        compare(100,320);
        // Mirrored indexed row, crossing texture-cache lines.
        command[32+:32]=420;command[192+:32]=127*65536;command[224+:32]=65536;
        command[256+:32]=-65536;submit();
        for(i=0;i<320;i=i+1) begin
            x=127-i;if(x<0)x=0;
            value=read_memory(32'h38200000+(128+x)*8);
            expected[420+i]=value[31:0];
        end
        compare(420,320);
        // RGBA source alpha-over. Reference uses literal rounded integer divides.
        command=0;command[1:0]=2;command[10:8]=3;
        command[32+:32]=100;command[64+:32]=64;
        command[96+:32]=32'h38401000;command[128+:32]=64;command[160+:32]=1;
        command[256+:32]=65536;submit();
        for(i=0;i<64;i=i+1) begin
            a=i*4+3;
            for(k=0;k<4;k=k+1) begin
                n=expected[100+i][k*8+:8];x=k==3?a:i*4+k;
                expected[100+i][k*8+:8]=((x*a+127)/255)+n-((n*a+127)/255);
            end
        end
        compare(100,64);
        // All four lines of this row remain resident, not just its last line.
        command[10:8]=7;
        reads_before=texture_reads;submit();
        if(texture_reads!=reads_before) $fatal(1,"repeated row missed resident texture lines");
        // Same cache indices, different tags: must refetch all four lines.
        command[96+:32]=32'h38402000;
        reads_before=texture_reads;submit();
        if(texture_reads!=reads_before+4) $fatal(1,"texture cache tag collision was not refetched");
        for(i=0;i<64;i=i+1) expected[100+i]={8'(i*4+3),8'(i*4+2),8'(i*4+1),8'(i*4)};
        compare(100,64);
        // Uploads can replace data at the same physical address next frame.
        texture_xor=8'h80;
        @(negedge clk);invalidate=1;@(negedge clk);invalidate=0;
        reads_before=texture_reads;submit();
        if(texture_reads!=reads_before+4) $fatal(1,"frame invalidation retained stale texture data");
        for(i=0;i<64;i=i+1) expected[100+i]=expected[100+i]^32'h80808080;
        compare(100,64);
        // Cold reads of the final refill beat, all byte lanes, changing V,
        // signed coordinate wrap and clamping. Compare against scalar samples
        // rather than another command taking the same optimized path.
        for(sampling_kind=1;sampling_kind<=2;sampling_kind=sampling_kind+1)
        for(sampling_case=0;sampling_case<4;sampling_case=sampling_case+1) begin
            @(negedge clk);invalidate=1;@(negedge clk);invalidate=0;
            test_u=sampling_kind==1?63*65536:15*65536;
            test_v=0;test_du=-65536;test_dv=0;
            case(sampling_case)
                1: begin test_u=-65536;test_v=-65536;test_du=32768;test_dv=65536;end
                2: begin test_u=32'h7ffff000;test_v=32'h7ffff000;test_du=65536;test_dv=65536;end
                3: begin test_u=32'h80001000;test_v=32'h80001000;test_du=-65536;test_dv=-65536;end
            endcase
            command=0;command[1:0]=sampling_kind;command[10:8]=7;
            command[32+:32]=1000;command[64+:32]=96;
            command[96+:32]=32'h38403000;command[128+:32]=64;command[160+:32]=4;
            command[192+:32]=test_u;command[224+:32]=test_v;
            command[256+:32]=test_du;command[288+:32]=test_dv;
            command[320+:32]=32'h38200000;submit();
            for(i=0;i<96;i=i+1) begin
                test_x=test_u>>>16;test_y=test_v>>>16;
                if(test_x<0)test_x=0;if(test_x>63)test_x=63;
                if(test_y<0)test_y=0;if(test_y>3)test_y=3;
                index=test_y*64+test_x;
                value=read_memory(32'h38403000+(sampling_kind==2?index*4:index));
                if(sampling_kind==1) begin
                    value=read_memory(32'h38200000+value[7:0]*8);
                    expected[1000+i]={value[31:24],value[55:32]};
                end else expected[1000+i]=value[31:0];
                test_u=test_u+test_du;test_v=test_v+test_dv;
            end
            compare(1000,96);
        end
        // Fast transparent/opaque paths must preserve RGBA for every blend
        // mode, including nonzero raw RGB behind a zero-alpha source.
        for(blend_mode=0;blend_mode<8;blend_mode=blend_mode+1) begin
            for(variant=0;variant<4;variant=variant+1) begin
                reference_mode=blend_mode;
                case(variant)
                    0: reference_source=64'h00c0804000000000;
                    1: reference_source=64'h00c08040ffb07030;
                    2: reference_source=64'h00c0804080604020;
                    3: reference_source=0;
                endcase
                reference_destination=32'h91735539;#1;
                command=0;command[10:8]=7;command[32+:32]=800;command[64+:32]=64;
                command[352+:64]=64'h0073553991000000;submit();
                command[10:8]=blend_mode;command[352+:64]=reference_source;
                cycles_before=cycle;submit();
                if((variant==3 && blend_mode!=7 || variant==1 && blend_mode>=2 && blend_mode<=4 || blend_mode==7) && cycle-cycles_before>210)
                    $fatal(1,"fast blend did not bypass pipeline mode=%d variant=%d cycles=%d",blend_mode,variant,cycle-cycles_before);
                if(cycle-cycles_before>330)
                    $fatal(1,"blend pipeline has an extra wait cycle: %d",cycle-cycles_before);
                for(i=800;i<864;i=i+1) expected[i]=reference_result;
                compare(800,64);
            end
        end
        // Warm-cache indexed replacement must use six cycles/pixel, plus
        // command setup and the initial row multiply. Also check each color
        // so an early palette read cannot silently select the previous index.
        command=0;command[1:0]=1;command[10:8]=7;
        command[32+:32]=1600;command[64+:32]=256;
        command[96+:32]=32'h38403000;command[128+:32]=256;command[160+:32]=1;
        command[256+:32]=65536;command[320+:32]=32'h38200000;
        submit();cycles_before=cycle;submit();
        if(cycle-cycles_before>1550)
            $fatal(1,"indexed palette pipeline has an extra wait cycle: %d",cycle-cycles_before);
        $display("Warm indexed row: %0d cycles for 256 pixels",cycle-cycles_before);
        for(i=0;i<256;i=i+1) begin
            value=read_memory(32'h38403000+i);
            value=read_memory(32'h38200000+value[7:0]*8);
            expected[1600+i]={value[31:24],value[55:32]};
        end
        compare(1600,256);
        // Compare grouped packets against separate rows for every blend/kind,
        // partial groups, exact nonuniform V, mirrored U and transparent pixels.
        for(grouped_kind=0;grouped_kind<3;grouped_kind=grouped_kind+1)
        for(grouped_mode=0;grouped_mode<8;grouped_mode=grouped_mode+1)
        for(grouped_rows=1;grouped_rows<=4;grouped_rows=grouped_rows+1) begin
            command=0;command[1:0]=3;command[32+:32]=3200;command[64+:32]=2880;submit();
            command=0;command[1:0]=grouped_kind;command[10:8]=grouped_mode;
            command[5:4]=grouped_rows-1;command[32+:32]=3200;command[64+:32]=17;
            command[96+:32]=32'h38400000;command[128+:32]=16;command[160+:32]=16;
            command[192+:32]=18*65536;command[256+:32]=-65536;
            command[224+:32]=7*65536+123;command[416+:32]=0;
            command[448+:32]=3*65536-1;command[480+:32]=15*65536;
            command[320+:32]=32'h38200000;command[352+:64]=grouped_rows==2?64'd0:64'h00c0804080604020;
            grouped_command=command;submit();
            for(j=0;j<grouped_rows;j=j+1) begin
                command=grouped_command;command[5:4]=0;
                command[32+:32]=4800+j*320;
                if(j!=0) command[224+:32]=grouped_command[384+j*32+:32];
                submit();
                for(i=0;i<17;i=i+1) begin
                    @(negedge clk);peek_address=4800+j*320+i;
                    @(posedge clk);#1;expected[3200+j*320+i]=peek_data;
                end
                compare(3200+j*320,17);
            end
        end
        // Last legal grouped pixel, then grouped overflow/reserved-bit rejection.
        command=0;command[5:4]=3;command[10:8]=7;
        command[32+:32]=76799-960;command[64+:32]=1;
        command[352+:64]=64'h00123456ff000000;submit();
        expected[76799]=32'hff123456;compare(76799,1);
        for(j=0;j<5;j=j+1) begin
            @(negedge clk);invalidate=1;@(negedge clk);invalidate=0;
            command=0;command[5:4]=3;command[32+:32]=76799-960;command[64+:32]=1;
            case(j)
                0: command[32+:32]=76799-959;
                1: command[64+:32]=321;
                2: command[1:0]=3;
                3: command[2]=1;
                4: command[6]=1;
            endcase
            @(negedge clk);command_valid=1;@(negedge clk);command_valid=0;
            if(!fault || !ready) $fatal(1,"invalid grouped command accepted case %d",j);
        end
        @(negedge clk);invalidate=1;@(negedge clk);invalidate=0;
        // A malformed destination is rejected without wrapping into block RAM.
        command=0;command[32+:32]=32'h10000000;command[64+:32]=1;
        @(negedge clk);command_valid=1;@(negedge clk);command_valid=0;
        if(!fault || !ready) $fatal(1,"invalid command was not rejected");
        $display("PASS: FPGA raster pixels, cache reuse/collisions/invalidation, backpressure and bounds; cycles=%0d texture bursts=%0d",cycle,texture_reads);
        $finish;
    end
endmodule
