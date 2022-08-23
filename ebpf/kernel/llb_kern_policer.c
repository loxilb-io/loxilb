/*
 *  llb_kern_policer.c: LoxiLB eBPF Policer Processing Implementation
 *  Copyright (C) 2022,  NetLOX <www.netlox.io>
 * 
 *  SPDX-License-Identifier: GPL-2.0 
 */
#define USECS_IN_SEC   (1000*1000)
#define NSECS_IN_USEC  (1000)

/* The intent here is to make this function non-inline
 * to keep code size in check
 */
static int
do_dp_policer(void *ctx, struct xfi *xf)
{
  struct dp_pol_tact *pla;
  int ret = 0;
  __u64 ts_now;
  __u64 ts_last;
  __u32 ntoks;
  __u32 inbytes;
  __u64 acc_toks;
  __u64 usecs_elapsed;

  ts_now = bpf_ktime_get_ns();

  pla = bpf_map_lookup_elem(&polx_map, &xf->qm.polid);
  if (!pla) { /*|| pla->ca.act_type != DP_SET_DO_POLICER) { */
    return 0;
  }

  inbytes = xf->pm.l3_len;

  bpf_spin_lock(&pla->lock);

  /* Calculate and add tokens to CBS */
  ts_last = pla->pol.lastc_uts;
  pla->pol.lastc_uts = ts_now;

  usecs_elapsed = (ts_now - ts_last)/NSECS_IN_USEC;
  acc_toks = pla->pol.toksc_pus * usecs_elapsed;
  if (acc_toks > 0) {
    if (pla->pol.cbs > pla->pol.tok_c) {
      ntoks = pla->pol.cbs - pla->pol.tok_c;  
      if (acc_toks > ntoks) {
        acc_toks -= ntoks;
        pla->pol.tok_c += ntoks;
      } else {
        pla->pol.tok_c += acc_toks;
        acc_toks = 0;
      }
    }
  } else {
    /* No tokens were added so we revert to last timestamp when tokens
     * were collected
     */
    pla->pol.lastc_uts = ts_last;
  }

  /* Calculate and add tokens to EBS */
  ts_last = pla->pol.laste_uts;
  pla->pol.laste_uts = ts_now;

  usecs_elapsed = (ts_now - ts_last)/NSECS_IN_USEC;
  acc_toks = pla->pol.tokse_pus * usecs_elapsed;
  if (acc_toks) {
    if (pla->pol.ebs > pla->pol.tok_e) {
      ntoks = pla->pol.ebs - pla->pol.tok_e;
      if (acc_toks > ntoks) {
        acc_toks -= ntoks;
        pla->pol.tok_e += ntoks;
      } else {
        pla->pol.tok_e += acc_toks;
        acc_toks = 0;
      }
    }
  } else {
    /* No tokens were added so we revert to last timestamp when tokens
     * were collected
     */
    pla->pol.laste_uts = ts_last;
  }

  if (pla->pol.color_aware == 0) {
    /* Color-blind mode */
    if (pla->pol.tok_e < inbytes) {
      xf->qm.ocol = LLB_PIPE_COL_RED;
    } else if (pla->pol.tok_c < inbytes) {
      xf->qm.ocol = LLB_PIPE_COL_YELLOW;
      pla->pol.tok_e -= inbytes;
    } else {
      pla->pol.tok_c -= inbytes;
      pla->pol.tok_e -= inbytes;
      xf->qm.ocol = LLB_PIPE_COL_GREEN;
    }
  } else {
    /* Color-aware mode */
    if (xf->qm.icol == LLB_PIPE_COL_NONE) {
      ret = -1;
      goto out;
    }

    if (xf->qm.icol == LLB_PIPE_COL_RED) {
      xf->qm.ocol = LLB_PIPE_COL_RED;
      goto out;
    }

    if (pla->pol.tok_e < inbytes) {
      xf->qm.ocol = LLB_PIPE_COL_RED;
    } else if (pla->pol.tok_c < inbytes) {
      if (xf->qm.icol == LLB_PIPE_COL_GREEN) {
        xf->qm.ocol = LLB_PIPE_COL_YELLOW;
      } else {
        xf->qm.ocol = xf->qm.icol;
      }
      pla->pol.tok_e -= inbytes;
    } else {
      pla->pol.tok_c -= inbytes;
      pla->pol.tok_e -= inbytes;
      xf->qm.ocol = xf->qm.icol;
    }
  }

out:
  if (pla->pol.drop_prio < xf->qm.ocol) { 
    ret = 1;
    pla->pol.ps.drop_packets += 1;
    LLBS_PPLN_DROP(xf);
  } else {
    pla->pol.ps.pass_packets += 1;
  }
  bpf_spin_unlock(&pla->lock); 
 
  return ret;
}
