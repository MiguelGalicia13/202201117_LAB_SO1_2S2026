#!/bin/bash
gcloud container clusters resize cluster-p3-202201117 \
    --node-pool=default-pool \
    --num-nodes=2 \
    --zone=us-central1-a
    
gcloud compute instances start zot-registry --zone=us-central1-a 