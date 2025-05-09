#!/bin/sh

# https://github.com/bugst/go-serial/blob/master/enumerator/usb_linux.go#L36

set -e
set -x

# Find all of the TTYs available on the system with names we might be interested in
ttys=$(ls /sys/class/tty | egrep '(ttyS|ttyHS|ttyUSB|ttyACM|ttyAMA|rfcomm|ttyO|ttymxc)[0-9]{1,3}')

for tty in $ttys; do
    # follow the symlink to find the real device
    realDevice=$(readlink -f /sys/class/tty/$tty/device)

    # what subsystem is it?
    subsystem=$(basename $(readlink -f $realDevice/subsystem))

    # locate the directory where the usb information is
    usbdir=""
    if [ "$subsystem" = "usb-serial" ]; then
        # usb-serial is two levels up from the tty
        usbdir=$(dirname $(dirname $realDevice))
    elif [ "$subsystem" = "usb" ]; then
        # regular usb is one level up from the tty
        usbdir=$(dirname $realDevice)
    else
        # we don't care about this device
        continue
    fi

    productId=$(cat $usbdir/idProduct)
    vendorId=$(cat $usbdir/idVendor)

    snippetFile="$vendorId:$productId.yaml"

    if [ -f "$snippetFile" ]; then
        sed "s/DEVNODE/\/dev\/$tty/" $snippetFile
    fi
done
