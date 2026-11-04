#!/usr/bin/env python3

import argparse
import json
import requests
import sys

requests.packages.urllib3.disable_warnings()

DEFAULT_ROLLOVER_SIZE = "300gb"
DEFAULT_MIN_DOC_COUNT = 200000000
DEFAULT_COOLDOWN = "6h"


class OpenSearchSetup:
    def __init__(self, host): #, user, password):
        self.host = host.rstrip("/")
        #self.auth = (user, password)

    def put(self, path, payload):
        url = f"{self.host}{path}"

        r = requests.put(
            url,
            #auth=self.auth,
            headers={"Content-Type": "application/json"},
            data=json.dumps(payload),
            verify=False,
        )

        if not r.ok:
            print(f"ERROR: PUT {url}")
            print(r.text)
            sys.exit(1)

        print(f"SUCCESS: PUT {path}")
        print(r.text)
        print()

    def create_policy(self, alias, rollover_size, min_doc_count, cooldown):
        policy_name = f"{alias}-rollover-policy"

        payload = {
            "policy": {
                "description": f"{alias} rollover policy",
                "default_state": "hot",
                "states": [
                    {
                        "name": "hot",
                        "actions": [
                            {
                                "rollover": {
                                    "min_size": rollover_size,
                                    "min_doc_count": min_doc_count,
                                }
                            }
                        ],
                        "transitions": [{"state_name": "cooldown"}],
                    },
                    {
                        "name": "cooldown",
                        "actions": [],
                        "transitions": [
                            {
                                "state_name": "locked",
                                "conditions": {"min_index_age": cooldown},
                            }
                        ],
                    },
                    {
                        "name": "locked",
                        "actions": [
                            {"read_only": {}},
                            {"force_merge": {"max_num_segments": 1}},
                        ],
                        "transitions": [],
                    },
                ],
                "ism_template": [{"index_patterns": [f"{alias}-*"], "priority": 100}],
            }
        }

        self.put(
            f"/_plugins/_ism/policies/{policy_name}",
            payload,
        )

        return policy_name

    def create_template(self, alias, policy_name):
        template_name = f"{alias}-template"

        payload = {
            "index_patterns": [f"{alias}-*"],
            "template": {
                "settings": {
                    "number_of_shards": 5,
                    "number_of_replicas": 0,
                    "refresh_interval": "60s",
                    
                    "plugins.index_state_management.policy_id": policy_name,
                    "plugins.index_state_management.rollover_alias": alias,
                }
            },
        }

        self.put(
            f"/_index_template/{template_name}",
            payload,
        )

    def create_bootstrap_index(self, alias):
        initial_index = f"{alias}-001"

        payload = {"aliases": {alias: {"is_write_index": True}}}

        self.put(
            f"/{initial_index}",
            payload,
        )

    def configure(
        self,
        alias,
        rollover_size,
        min_doc_count,
        cooldown,
    ):
        print(f"Configuring alias: {alias}")
        print(f"Rollover size: {rollover_size}")
        print(f"Min document count: {min_doc_count:,}")
        print(f"Cooldown: {cooldown}")
        print()

        policy_name = self.create_policy(
            alias,
            rollover_size,
            min_doc_count,
            cooldown,
        )

        self.create_template(
            alias,
            policy_name,
        )

        self.create_bootstrap_index(alias)

        print("Done.")
        print()
        print(f"Write alias: {alias}")
        print(f"Initial index: {alias}-001")
        print(f"Policy: {policy_name}")


def main():
    parser = argparse.ArgumentParser()

    parser.add_argument("--host", required=True, default="http://localhost:9200", help="url (with port) of the OpenSearch cluster")
    #parser.add_argument("--user", required=True)
    #parser.add_argument("--password", required=True)
    parser.add_argument("--alias", required=True, help="name of the index alias to create")
    parser.add_argument("--rollover-size", default=DEFAULT_ROLLOVER_SIZE, help="rollvover size")
    parser.add_argument("--min-doc-count", type=int, default=DEFAULT_MIN_DOC_COUNT, help="minimum number of documents per index")
    parser.add_argument("--cooldown", default=DEFAULT_COOLDOWN, help="cooldown period")

    args = parser.parse_args()

    OpenSearchSetup(
        args.host,
        # args.user,
        # args.password,
    ).configure(
        alias=args.alias,
        rollover_size=args.rollover_size,
        min_doc_count=args.min_doc_count,
        cooldown=args.cooldown,
    )


if __name__ == "__main__":
    main()
