#!/bin/bash
echo -e "\n DAEMON: "
ps aux | grep daemon
echo -e "\n =========================================== \n"
echo -e "REDIS KEYS: \n"
docker exec valkey-db valkey-cli KEYS '*'
echo -e "\n =========================================== \n"
echo -n "REDIS GET TOTAL RAM: " 
docker exec valkey-db valkey-cli GET system:total_ram
