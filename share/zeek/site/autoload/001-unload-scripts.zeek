##! Do NOT customize this file. Use autoload/100-default.zeek or a custom
##! script in autoload/ instead.
##!
##! This file WILL be overwritten when upgrading or reinstalling!


# @unload specifies a Zeek script that we don't want to load (so subsequent attempts to load will be skipped). 
# However, if the specified script has already been loaded, then this directive has no affect.
# https://docs.zeek.org/en/master/script-reference/directives.html#unload

# Disable MD5 and SHA1 hashing for all files.
@unload frameworks/files/hash-all-files

# Disable detecting SHA1 sums in Team Cymru's Malware Hash Registry.
@unload frameworks/files/detect-MHR
