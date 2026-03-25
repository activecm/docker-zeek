##! Do NOT customize this file. Use autoload/100-default.zeek or a custom
##! script in autoload/ instead.
##!
##! This file WILL be overwritten when upgrading or reinstalling!

# This is important for accurate connection info. More info here: 
# https://www.activecountermeasures.com/fixing-bro-zeeks-long-connection-detection-problem/
redef tcp_inactivity_timeout = 60 min;
