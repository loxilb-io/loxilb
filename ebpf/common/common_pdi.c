/*
 * Copyright 2022 <Dip J, dipj@netlox.io>
 *
 * SPDX short identifier: BSD-3-Clause
 */
#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <pthread.h>
#include <linux/types.h>
#include <arpa/inet.h>
#include "uthash.h"
#include "pdi.h"

struct pdi_map *
pdi_map_alloc(pdi_add_map_op_t add_map, pdi_del_map_op_t del_map)
{
  struct pdi_map *map = calloc(1, sizeof(struct pdi_map));
  map->pdi_add_map = add_map;
  map->pdi_del_map = del_map;

  return map;
}

void
pdi_key2str(struct pdi_key *key, char *fstr)
{
  int l = 0;

  PDI_MATCH_PRINT(&key->dest, "dest", fstr, l, none);
  PDI_MATCH_PRINT(&key->source, "source", fstr, l, none);
  PDI_RMATCH_PRINT(&key->dport, "dport", fstr, l, none);
  PDI_RMATCH_PRINT(&key->dport, "sport", fstr, l, none);
  PDI_MATCH_PRINT(&key->qos, "qos", fstr, l, none);
  PDI_MATCH_PRINT(&key->protocol, "prot", fstr, l, none);
  PDI_MATCH_PRINT(&key->dir, "dir", fstr, l, none);
  PDI_MATCH_PRINT(&key->ident, "ident", fstr, l, none);
}

void
pdi_rule2str(struct pdi_rule *node)
{
  char fmtstr[1000] = { 0 };

  if (1) {
    pdi_key2str(&node->key, fmtstr);
    printf("(%s)%d\n", fmtstr, node->data.pref);
  }
}

void
pdi_rules2str(struct pdi_map *map)
{
  struct pdi_rule *node = map->head;

  printf("#### Rules ####\n");
  while (node) {
    pdi_rule2str(node);
    node = node->next;
  }
  printf("##############\n");
}

int
pdi_rule_insert(struct pdi_map *map, struct pdi_rule *new)
{
  struct pdi_rule *prev =  NULL;
  struct pdi_rule *node;
  uint32_t pref = new->data.pref;

  PDI_MAP_LOCK(map);

  node = map->head;

  while (node) {
    if (pref > node->data.pref) {
      if (prev) {
        prev->next = new;
        new->next = node;
      } else {
        map->head = new;
        new->next = node;
      }

      PDI_MAP_ULOCK(map);
      return 0;
    }

    if (pref == node->data.pref)  {
      if (PDI_KEY_EQ(&new->key, &node->key)) {
        PDI_MAP_ULOCK(map);
        return -EEXIST;
      } 
    }
    prev = node;
    node = node->next;
  }

  if (prev) {
    prev->next = new;
    new->next = node;
  } else {
    map->head = new;
    new->next = node;
  }

  PDI_MAP_ULOCK(map);

  return 0;
}

struct pdi_rule *
pdi_rule_delete__(struct pdi_map *map, struct pdi_key *key, uint32_t pref)
{
  struct pdi_rule *prev =  NULL;
  struct pdi_rule *node;

  node = map->head;

  while (node) {
    if (pref == node->data.pref)  {
      if (PDI_KEY_EQ(key, &node->key)) {
        if (prev) {
          prev->next = node->next;
        } else {
          map->head = node->next;
        }
        return node;
      } 
    }
    prev = node;
    node = node->next;
  }

  return NULL;
}

int
pdi_rule_delete(struct pdi_map *map, struct pdi_key *key, uint32_t pref)
{
  struct pdi_rule *node = NULL;
  struct pdi_val *val, *tmp;

  PDI_MAP_LOCK(map);

  node = pdi_rule_delete__(map, key, pref);
  if (node != NULL) {
    printf("Deleting....\n");
    pdi_rule2str(node);
    HASH_ITER(hh, node->hash, val, tmp) {
      HASH_DEL(node->hash, val);
      if (map->pdi_del_map) {
        map->pdi_del_map(&val->val);
      }
      free(val);
      printf("Hash del\n");
    }
    free(node);
    PDI_MAP_ULOCK(map);

    return 0;
  }

  PDI_MAP_ULOCK(map);
  return -1;
}

struct pdi_rule *
pdi_rule_get__(struct pdi_map *map, struct pdi_key *val)
{
  struct pdi_rule *node = map->head;

  while (node) {
    //pdi_rule2str(node);
    if (PDI_PKEY_EQ(val, &node->key)) {
      return node;
    } 
    node = node->next;
  }
  return NULL;
}

int
pdi_add_val(struct pdi_map *map, struct pdi_key *kval)
{
  struct pdi_val *hval = NULL;
  struct pdi_rule *rule = NULL;

  PDI_MAP_LOCK(map);

  rule = pdi_rule_get__(map, kval);
  if (rule != NULL) {
    printf("Found match --\n");
    pdi_rule2str(rule);

    HASH_FIND(hh, rule->hash, kval, sizeof(struct pdi_key), hval);
    if (hval) {
      printf("hval exists\n");
      if (map->pdi_add_map) {
        map->pdi_add_map(kval, &rule->data, sizeof(rule->data));
      }
      PDI_MAP_ULOCK(map);
      return -EEXIST;
    }

    hval = calloc(1, sizeof(*hval));
    memcpy(&hval->val, kval, sizeof(*kval));
    hval->r = rule;
    HASH_ADD(hh, rule->hash, val, sizeof(struct pdi_key), hval);
    PDI_MAP_ULOCK(map);
    return 0;
  }

  PDI_MAP_ULOCK(map);

  return -1;
}

int
pdi_del_val(struct pdi_map *map, struct pdi_key *kval)
{
  struct pdi_val *hval = NULL;
  struct pdi_rule *rule = NULL;

  PDI_MAP_LOCK(map);

  rule = pdi_rule_get__(map, kval);
  if (rule != NULL) {
    printf("Found match --\n");
    pdi_rule2str(rule);

    HASH_FIND(hh, rule->hash, kval, sizeof(struct pdi_key), hval);
    if (hval == NULL) {
      printf("hval does not exist\n");
      PDI_MAP_ULOCK(map);
      return -EINVAL;
    }

    HASH_DEL(rule->hash, hval);
    PDI_MAP_ULOCK(map);
    return 0;
  }

  PDI_MAP_ULOCK(map);
  return -1;
}

static int
pdi_val_expired(struct pdi_val *v)
{
  // TODO 
  return 0;
}

void
pdi_map_run(struct pdi_map *map)
{
  struct pdi_rule *node;
  struct pdi_val *val, *tmp;
  char fmtstr[512] = { 0 };

  PDI_MAP_LOCK(map);

  node = map->head;

  while (node) {
    HASH_ITER(hh, node->hash, val, tmp) {
      if (pdi_val_expired(val)) {
        HASH_DEL(node->hash, val);
        if (map->pdi_del_map) {
          map->pdi_del_map(&val->val);
        }
        pdi_key2str(&val->val, fmtstr);
        printf("Expired entry %s\n", fmtstr);
        free(val);
      }
    }
    node = node->next;
  }
  PDI_MAP_ULOCK(map);
}

int
pdi_unit_test(void)
{
  struct pdi_map *map;
  int r = 0;

  map = pdi_map_alloc(NULL, NULL);

  struct pdi_rule *new = calloc(1, sizeof(struct pdi_rule));
  if (new) {
    PDI_MATCH_INIT(&new->key.dest, 0x0a0a0a0a, 0xffffff00);
    PDI_RMATCH_INIT(&new->key.dport, 1, 100, 200); 
    r = pdi_rule_insert(map, new);
    if (r != 0) {
      printf("Insert fail1\n");
      exit(0);
    }
  }


  struct pdi_rule *new1 = calloc(1, sizeof(struct pdi_rule));
  if (new1) {
    memcpy(new1, new, sizeof(*new));
    new1->data.pref = 100;
    r = pdi_rule_insert(map, new1);
    if (r != 0) {
     printf("Insert fail2\n");
     exit(0);
    }
  }


  struct pdi_rule *new2 = calloc(1, sizeof(struct pdi_rule));
  if (new2) {
    PDI_MATCH_INIT(&new2->key.dest, 0x0a0a0a0a, 0xffffff00);
    PDI_RMATCH_INIT(&new2->key.dport, 0, 100, 0xffff); 
    r = pdi_rule_insert(map, new2);
    if (r != 0) {
      printf("Insert fail3\n");
      exit(0);
    }

    r = pdi_rule_insert(map, new2);
    if (r == 0) {
      printf("Insert fail4\n");
      exit(0);
    }
  }

  if (pdi_rule_delete(map, &new1->key, 100) != 0) {
    // Free //
    printf("Delete fail4\n");
    exit(0);
  }

  struct pdi_rule *new4 = calloc(1, sizeof(struct pdi_rule));
  if (new4) {
    PDI_MATCH_INIT(&new4->key.dest, 0x0a0a0a0a, 0xffffff00);
    PDI_MATCH_INIT(&new4->key.source, 0x0b0b0b00, 0xffffff00);
    PDI_RMATCH_INIT(&new4->key.dport, 1, 500, 600); 
    PDI_RMATCH_INIT(&new4->key.sport, 1, 500, 600); 
    r = pdi_rule_insert(map, new4);
    if (r != 0) {
      printf("Insert fail1\n");
      exit(0);
    }
  }

  pdi_rules2str(map);

  if (1) {
    struct pdi_key key =  { 0 } ;
    PDI_VAL_INIT(&key.source, 0x0b0b0b0b);
    PDI_VAL_INIT(&key.dest, 0x0a0a0a0a);
    PDI_RVAL_INIT(&key.dport, 501);
    PDI_RVAL_INIT(&key.sport, 501);
    if (pdi_add_val(map, &key) != 0) {
      printf("Failed to add pdi val1\n");
    }
  }

  if (1) {
    struct pdi_key key =  { 0 } ;
    PDI_VAL_INIT(&key.source, 0x0b0b0b0b);
    PDI_VAL_INIT(&key.dest, 0x0a0a0a0a);
    PDI_RVAL_INIT(&key.dport, 502);
    PDI_RVAL_INIT(&key.sport, 502);
    if (pdi_add_val(map, &key) != 0) {
      printf("Failed to add pdi val2\n");
    }
  }

  if (pdi_rule_delete(map, &new4->key, 0) != 0) {
     printf("Failed delete--%d\n", __LINE__);
  }

  return 0;
}
