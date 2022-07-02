/*
 * Copyright 2022 <Dip J, dipj@netlox.io>
 *
 * SPDX short identifier: BSD-3-Clause
 */
#ifndef __PDI_H__
#define __PDI_H__

#define none(x)  (x)

#define PDI_TYPEDEF(sz) pdi_tup##sz##_t
#define PDI_TYPEDEF_R(sz) pdi_tupr##sz##_t

#define PDI_DEF_FIELD(sz)       \
typedef struct pdi_tup##sz##_ { \
    __u##sz val;                \
    __u##sz valid;              \
} PDI_TYPEDEF(sz)

#define PDI_DEF_RANGE_FIELD(sz)   \
typedef struct pdi_tupwr##sz##_ { \
    uint32_t has_range;           \
    union {                     \
      struct pdi_tup##sz##r_  { \
      __u##sz min;              \
      __u##sz max;              \
      }r;                       \
      struct pdi_tup##sz##v_ {  \
        __u##sz val;            \
        __u##sz valid;          \
      }v;                       \
    }u;                         \
}pdi_tupr##sz##_t;

PDI_DEF_FIELD(64);
PDI_DEF_FIELD(32);
PDI_DEF_RANGE_FIELD(16);
PDI_DEF_FIELD(16);
PDI_DEF_FIELD(8);

#define PDI_MATCH(v1, v2) \
(((v2)->valid == 0) || ((v2)->valid && (((v1)->val & (v2)->valid) == (v2)->val)))

#define PDI_RMATCH(v1, v2) \
(((v2)->has_range && ((v1)->u.v.val >= (v2)->u.r.min && (v1)->u.v.val <= (v2)->u.r.max)) || \
 (((v2)->has_range == 0 ) && (((v2)->u.v.valid == 0) || ((v2)->u.v.valid && (((v1)->u.v.val & (v2)->u.v.valid) == (v2)->u.v.val)))))

#define PDI_MATCH_ALL(v1, v2) \
(((v2)->valid == (v1)->valid) && (((v1)->val & (v2)->valid) == (v2)->val))

#define PDI_RMATCH_ALL(v1, v2) \
((((v2)->has_range == (v1)->has_range) && ((v1)->u.v.val >= (v2)->u.r.min && (v1)->u.v.val <= (v2)->u.r.max)) || \
 (((v2)->has_range != 0) &&(((v2)->u.v.valid == (v1)->u.v.valid) && (((v1)->u.v.val & (v2)->u.v.valid) == (v2)->u.v.val))))

#define PDI_MATCH_PRINT(v1, kstr, fmtstr, l, cv)                   \
do {                                                               \
  if ((v1)->valid) {                                               \
    l += sprintf(fmtstr+l, "%s:0x%x,", kstr, cv((v1)->valid & (v1)->val)); \
  }                                                                \
} while(0)

#define PDI_RMATCH_PRINT(v1, kstr, fmtstr, l, cv)                  \
do {                                                               \
  if ((v1)->has_range) {                                           \
    l += sprintf(fmtstr+l, "%s:%d-%d,", kstr,cv((v1)->u.r.min), cv((v1)->u.r.max));   \
  }                                                                \
  else {                                                           \
    l += sprintf(fmtstr+l, "%s:0x%x,", kstr, cv((v1)->u.v.valid & (v1)->u.v.val)); \
  }                                                                \
} while(0)

#define PDI_MATCH_INIT(v1, v, vld)           \
do {                                         \
  (v1)->valid = vld;                         \
  (v1)->val = (v) & (v1)->valid;             \
} while (0)

#define PDI_RMATCH_INIT(v1, hr, val1, val2)  \
do {                                         \
  if (hr) {                                  \
    (v1)->u.r.min = val1;                    \
    (v1)->u.r.max = val2;                    \
    (v1)->has_range = 1;                     \
  } else {                                   \
    (v1)->u.v.valid = val2;                  \
    (v1)->u.v.val = val1 & (v1)->u.v.valid;  \
    (v1)->has_range = 0;                     \
  }                                          \
} while (0)

#define PDI_VAL_INIT(v1, v)                  \
do {                                         \
  (v1)->valid = -1;                          \
  (v1)->val = (v);                           \
} while (0)

#define PDI_RVAL_INIT(v1, val1)              \
do {                                         \
    (v1)->u.v.valid = -1;                    \
    (v1)->u.v.val = val1;                    \
    (v1)->has_range = 0;                     \
} while (0)

#define PDI_MAP_LOCK(m) pthread_rwlock_wrlock(&m->lock)
#define PDI MAP_RLOCK(m) pthread_rwlock_rdlock(&m->lock)
#define PDI_MAP_ULOCK(m) pthread_rwlock_unlock(&m->lock)

struct pdi_map {
  pthread_rwlock_t lock;
  struct pdi_rule *head;
  int (*pdi_add_map)(void *key, void *val, size_t sz);
  int (*pdi_del_map)(void *key);
};

typedef int (*pdi_add_map_op_t)(void *key, void *val, size_t sz);
typedef int (*pdi_del_map_op_t)(void *key);

struct pdi_key {
    PDI_TYPEDEF(32)    dest;
    PDI_TYPEDEF(32)    source;
    PDI_TYPEDEF_R(16)  dport;
    PDI_TYPEDEF_R(16)  sport;
    PDI_TYPEDEF(16)    qos;
    PDI_TYPEDEF(8)     protocol;
    PDI_TYPEDEF(8)     dir;
    PDI_TYPEDEF(32)    ident;
};

#define PDI_FAR_SET_QFI    0x1
#define PDI_FAR_SET_POL    0x2
#define PDI_FAR_SET_GBR    0x4
#define PDI_FAR_SET_DROP   0x8
#define PDI_FAR_SET_MIRR   0x10
#define PDI_FAR_SET_FWD    0x20
#define PDI_FAR_SET_ANTID  0x40
#define PDI_FAR_RM_TID     0x80

struct pdi_far {
  uint32_t qfi; 
  uint16_t polid;
  uint16_t qid;
  uint16_t mirrid;
  uint16_t res;
  uint32_t port;
  uint32_t teid;
};

struct pdi_data {
  uint32_t pref;
  uint32_t rid;
  struct pdi_far frd;
};

struct pdi_stats {
  uint64_t bytes;
  uint64_t packets;
};

struct pdi_val {
  struct pdi_key val;
  struct pdi_rule *r;
#define PDI_VAL_INACT_TO  (60000000000)
  uint64_t lts;
  UT_hash_handle hh;
};

struct pdi_rule {
  struct pdi_key key;
  struct pdi_data data;
  struct pdi_rule *next;
  struct pdi_val *hash;
};

#define PDI_KEY_EQ(v1, v2)                                  \
  ((PDI_MATCH_ALL(&(v1)->dest, &(v2)->dest)))         &&    \
  ((PDI_MATCH_ALL(&(v1)->source, &(v2)->source)))     &&    \
  ((PDI_RMATCH_ALL(&(v1)->dport, &(v2)->dport)))      &&    \
  ((PDI_RMATCH_ALL(&(v1)->sport, &(v2)->sport)))      &&    \
  ((PDI_MATCH_ALL(&(v1)->qos, &(v2)->qos)))           &&    \
  ((PDI_MATCH_ALL(&(v1)->dir, &(v2)->dir)))           &&    \
  ((PDI_MATCH_ALL(&(v1)->protocol, &(v2)->protocol))) &&    \
  ((PDI_MATCH_ALL(&(v1)->ident, &(v2)->ident)))

#define PDI_PKEY_EQ(v1, v2)                             \
  (((PDI_MATCH(&(v1)->dest, &(v2)->dest)))         &&   \
  ((PDI_MATCH(&(v1)->source, &(v2)->source)))     &&    \
  ((PDI_RMATCH(&(v1)->dport, &(v2)->dport)))      &&    \
  ((PDI_RMATCH(&(v1)->sport, &(v2)->sport)))      &&    \
  ((PDI_MATCH(&(v1)->qos, &(v2)->qos)))           &&    \
  ((PDI_MATCH(&(v1)->dir, &(v2)->dir)))           &&    \
  ((PDI_MATCH(&(v1)->protocol, &(v2)->protocol))) &&    \
  ((PDI_MATCH(&(v1)->ident, &(v2)->ident))))

#define PDI_PKEY_NTOH(v1)                          \
    (v1)->dest = htonl((v1)->dest);                \
    (v1)->source = htonl((v1)->source);            \
    (v1)->dport = htons((v1)->dport);              \
    (v1)->dest = htons((v1)->sport);               \
    (v1)->ident= htonl((v1)->ident);               \
    (v1)->qos = htons((v1)->qos);


#endif
