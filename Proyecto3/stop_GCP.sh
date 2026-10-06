#!/bin/bash
gcloud container clusters resize cluster-p3-202201117 \
    --node-pool=default-pool \
    --num-nodes=0 \
    --zone=us-central1-a

gcloud compute instances stop zot-registry --zone=us-central1-a 