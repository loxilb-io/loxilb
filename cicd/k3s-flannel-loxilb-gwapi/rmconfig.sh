#!/bin/bash
vagrant destroy -f master
vagrant destroy -f loxilb
rm -f k3s.yaml loxilb-ip loxilb-secret.yml master-ip minica-key.pem minica.pem node-token server.crt server.key
rm -rf cert/ certs/ loxilb.io/
