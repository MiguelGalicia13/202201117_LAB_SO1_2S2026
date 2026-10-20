#!/bin/bash
echo -e "\n=============== VUETA 1 ===================="
echo -e "Cluster de GKE y nodos activos \n"
kubectl get nodes -o wide
echo -e "\n----------------------------------------\n"
echo -e "Gateway y Rutas: \n"
kubectl get gateway,httproute -n proyecto3
echo -e "\n----------------------------------------\n"

