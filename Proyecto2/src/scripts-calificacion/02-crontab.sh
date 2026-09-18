#!/bin/bash 
echo -e "\n TABLA DOCKER \n"
docker ps --format "table {{.ID}}\t{{.Names}}\t{{.Status}}"
echo -e "\n =========================================== \n"
echo -e "Crontab: \n"
sudo crontab -l
echo -e "\n =========================================== \n"
echo -e "DOCKER CONTAINERS"
docker ps