#!/bin/sh

echo "starting jandaira server"
./jandaira & 

nginx -g 'daemon off;'