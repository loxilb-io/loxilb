#!/bin/bash
vagrant destroy -f master
vagrant destroy -f loxilb
vagrant destroy -f bastion
rm -f extIP
