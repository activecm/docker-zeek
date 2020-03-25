#!/bin/bash

# https://www.zeek.org/bro-workshop-2011/solutions/extending/index.html

pushd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null

FILENAME=__load__.zeek

echo "redef SSL::root_certs += {" > $FILENAME

for f in *.cer *.crt; do
    echo "Processing $f"
    echo -n "    [\"${f%.cer}\"] = \"" >> $FILENAME
    openssl x509 -in "$f" -inform PEM -outform DER | hexdump -v -e '1/1 "\\\x"' -e '1/1 "%02X"' >> $FILENAME
    echo '",' >> $FILENAME
done

echo "};" >> $FILENAME

popd >/dev/null

# https://securityonion.readthedocs.io/en/latest/zeek.html#custom-scripts
# mkdir /opt/bro/share/bro/custom-ca
# cp *.cer /opt/bro/share/bro/custom-ca
# cp cer2zeek.sh /opt/bro/share/bro/custom-ca
# /opt/bro/share/bro/custom-ca/cer2zeek.sh
# echo >> /opt/bro/share/bro/site/local.zeek
# echo "# Load custom CA" >> /opt/bro/share/bro/site/local.zeek
# echo "@load custom-ca" >> /opt/bro/share/bro/site/local.zeek
# so-zeek-restart || zeekctl deploy
