#!/bin/bash

package_image() {
    rm -rf build/bin/
    nix build
    mkdir -p build/bin
    cp result/bin/statefulset-affinity-injector build/bin/statefulset-affinity-injector
    docker build --no-cache --progress=plain -t "statefulset-affinity-injector" .
}

package_chart() {
    cd charts/
    helm package statefulset-affinity-injector
    cd ../
}


# Print usage
usage() {
    echo "Usage: $0 [image|chart]"
    exit 1
}

# Parse command-line arguments
if [ $# -ne 1 ]; then
    usage
fi

command="$1"

case "$command" in
    image)
        package_image
        ;;
    chart)
        package_chart
        ;;
    *)
        usage
        ;;
esac
