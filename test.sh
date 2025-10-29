#!/bin/bash

./mothership -ts-port :8001 &

sleep 1

./rover -id ROVER-01 -mothership localhost:8001 -interval 2s
