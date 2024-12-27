#!/bin/bash
vagrant destroy -f worker2
vagrant destroy -f worker1
vagrant destroy -f master
vagrant destroy -f llb1
vagrant destroy -f llb2
