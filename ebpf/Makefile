# SPDX-License-Identifier: (GPL-2.0 OR BSD-2-Clause)

kerninstalldir = $(shell pwd)/loxilb-kern
export kerninstalldir

KERN = $(wildcard kernel*)
KERN_CLEAN = $(addsuffix _clean,$(KERN))
KERN_INST = $(addsuffix _install,$(KERN))

.PHONY: clean $(KERN) $(KERN_CLEAN) $(KERN_INST)

all: $(KERN)
clean: $(KERN_CLEAN)
install: $(KERN_INST)

$(KERN):
	$(MAKE) -C $@

$(KERN_CLEAN):
	$(MAKE) -C $(subst _clean,,$@) clean

$(KERN_INST):
	@sudo rm -fr $(kerninstalldir)
	@mkdir -p $(kerninstalldir)
	@echo dp-release path : $(kerninstalldir)
	$(MAKE) -C $(subst _install,,$@) install
