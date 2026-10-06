project_open IKEMEN
create_timing_netlist
read_sdc
update_timing_netlist
report_timing -setup -npaths 20 -nworst 1 -detail full_path -file output_files/gpu-critical.rpt
delete_timing_netlist
project_close
