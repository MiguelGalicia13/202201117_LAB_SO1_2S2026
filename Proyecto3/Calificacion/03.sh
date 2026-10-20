echo -e "\n=============== VUETA 3 ===================="
echo -e "CRD'S \n"
kubectl get crd -n proyecto3
echo -e "\n Instancias declaradas \n"
kubectl get valkeyinstances,grafanainstances -n proyecto3
echo -e "\n----------------------------------------\n"
echo -e "\n Pods \n"
kubectl get pods -n proyecto3
echo -e "\n----------------------------------------\n"
echo -e "Persistencia de Valkey: \n"
kubectl exec -it valkeyinstance-sample-0 -n proyecto3 -- valkey-cli DBSIZE