#!/bin/bash
echo -e "\n=============== VUETA 2 ===================="
echo -e "HPA \n"
kubectl get hpa -n proyecto3
echo -e "\n Deploys \n"
kubectl get deployments -n proyecto3
echo -e "\n----------------------------------------\n"
echo -e "Logs de gRPC: \n"
kubectl logs -l app=api-rust --tail=1 -n proyecto3


