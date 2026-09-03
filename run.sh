#!/bin/bash

cd "$(dirname "$0")"

set -a
. .env
set +a

go run .
