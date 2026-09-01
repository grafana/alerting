#!/usr/bin/env python3
"""Seed a Loki instance with synthetic notification-history entries.

This lets you exercise the notification-history query endpoints (and the RBAC
filtering in particular) without running the full alerting pipeline. It writes
log lines in the exact shape the historian read path expects:

  * stream {from="notify-history-events"} - one line per notification, with
    structured metadata `rule_uids` (comma-separated) used by the RBAC filter.
  * stream {from="notify-history-alerts"} - one line per alert, with structured
    metadata `rule_uid` and label `__alert_rule_uid__`.

The rule UIDs you pass MUST match real AlertRule resources in Grafana, because
the RBAC filter lists accessible rules via the rules API and only keeps history
whose rule_uids intersect that set.

Example (two rules, one per folder, plus a "mixed" notification referencing both):

  python3 seed_notification_history.py \
      --loki-url http://localhost:3100 \
      --rule uid=RULE_A_UID,folder=FOLDER_A_UID,receiver="Team A webhook" \
      --rule uid=RULE_B_UID,folder=FOLDER_B_UID,receiver="Team B webhook" \
      --count 3

Then query as different users, e.g.:

  curl -s -u usera:pass -H 'Content-Type: application/json' \
    -X POST 'http://localhost:3000/apis/historian.alerting.grafana.app/v0alpha1/namespaces/default/notification/query' \
    -d '{"type":"entries"}' | jq '.entries[].ruleUIDs'
"""

import argparse
import json
import sys
import urllib.error
import urllib.request
import uuid as uuidlib
from datetime import datetime, timedelta, timezone


def rfc3339(dt: datetime) -> str:
    """Format a datetime as RFC3339 with a trailing Z (parseable by Go's time)."""
    return dt.astimezone(timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ")


def unix_nano(dt: datetime) -> int:
    return int(dt.timestamp() * 1_000_000_000)


def parse_rule(spec: str) -> dict:
    """Parse a --rule spec of the form key=value,key=value.

    Supported keys: uid (required), folder, receiver, alertname.
    """
    out = {"folder": "", "receiver": "Seeded contact point", "alertname": "SeededAlert"}
    for part in spec.split(","):
        part = part.strip()
        if not part:
            continue
        if "=" not in part:
            raise argparse.ArgumentTypeError(f"invalid rule field {part!r}, expected key=value")
        key, value = part.split("=", 1)
        key = key.strip()
        if key not in ("uid", "folder", "receiver", "alertname"):
            raise argparse.ArgumentTypeError(f"unknown rule key {key!r}")
        out[key] = value.strip()
    if not out.get("uid"):
        raise argparse.ArgumentTypeError(f"rule spec {spec!r} is missing required uid=...")
    return out


def notif_line(ts: datetime, uuid: str, rule_uids, folder_uids, receiver: str,
               status: str, error: str, group_labels: dict):
    """Build a (values-entry, ) tuple for the notify-history-events stream."""
    body = {
        "schemaVersion": 2,
        "uuid": uuid,
        "ruleUIDs": rule_uids,
        "folderUIDs": folder_uids,
        "receiver": receiver,
        "integration": "webhook",
        "integrationIdx": 0,
        "groupKey": "{}/{alertname=\"%s\"}" % group_labels.get("alertname", ""),
        "status": status,
        "groupLabels": group_labels,
        "alertCount": 1,
        "retry": False,
        "duration": 1_000_000,
        "pipelineTime": rfc3339(ts),
    }
    if error:
        body["error"] = error
    metadata = {
        "uuid": uuid,
        "receiver": receiver,
        "rule_uids": ",".join(rule_uids),
        "folder_uids": ",".join(folder_uids),
    }
    return [str(unix_nano(ts)), json.dumps(body), metadata]


def alert_line(ts: datetime, uuid: str, rule_uid: str, folder_uid: str,
               alertname: str, status: str):
    """Build a values-entry for the notify-history-alerts stream."""
    body = {
        "schemaVersion": 2,
        "uuid": uuid,
        "alertIndex": 0,
        "status": status,
        "labels": {"__alert_rule_uid__": rule_uid, "alertname": alertname},
        "annotations": {"__alert_rule_namespace_uid__": folder_uid},
        "startsAt": rfc3339(ts - timedelta(minutes=30)),
        "endsAt": rfc3339(ts - timedelta(minutes=5)),
    }
    metadata = {"uuid": uuid, "rule_uid": rule_uid, "folder_uid": folder_uid}
    return [str(unix_nano(ts)), json.dumps(body), metadata]


def push(loki_url: str, tenant: str, streams: list) -> None:
    payload = json.dumps({"streams": streams}).encode("utf-8")
    url = loki_url.rstrip("/") + "/loki/api/v1/push"
    req = urllib.request.Request(url, data=payload, method="POST")
    req.add_header("Content-Type", "application/json")
    if tenant:
        req.add_header("X-Scope-OrgID", tenant)
    try:
        with urllib.request.urlopen(req) as resp:
            if resp.status not in (204, 200):
                raise SystemExit(f"unexpected status from Loki: {resp.status}")
    except urllib.error.HTTPError as e:
        detail = e.read().decode("utf-8", "replace")
        raise SystemExit(f"Loki push failed ({e.code}): {detail}")
    except urllib.error.URLError as e:
        raise SystemExit(f"could not reach Loki at {url}: {e.reason}")


def main() -> int:
    ap = argparse.ArgumentParser(
        description="Seed Loki with synthetic notification-history entries.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    ap.add_argument("--loki-url", default="http://localhost:3100",
                    help="Base URL of the Loki instance (default: %(default)s)")
    ap.add_argument("--tenant", default="",
                    help="X-Scope-OrgID header value (match unified_alerting.notification_history loki_tenant_id if set)")
    ap.add_argument("--rule", action="append", type=parse_rule, required=True, metavar="SPEC",
                    help="Repeatable. Format: uid=<ruleUID>,folder=<folderUID>,receiver=<name>,alertname=<name>")
    ap.add_argument("--count", type=int, default=2,
                    help="Number of notifications to seed per rule (default: %(default)s)")
    ap.add_argument("--no-mixed", action="store_true",
                    help="Do not create a notification that references all rules at once")
    ap.add_argument("--dry-run", action="store_true",
                    help="Print the Loki push payload instead of sending it")
    args = ap.parse_args()

    now = datetime.now(timezone.utc)
    notif_values = []
    alert_values = []
    # Use a per-entry offset so every sample gets a distinct timestamp.
    tick = 0

    def next_ts():
        nonlocal tick
        tick += 1
        # Spread entries over the last hour, newest first.
        return now - timedelta(minutes=tick)

    for rule in args.rule:
        for i in range(args.count):
            ts = next_ts()
            u = str(uuidlib.uuid4())
            # Alternate success/error so counts-by-outcome has something to show.
            is_error = (i % 2 == 1)
            status = "firing" if not is_error else "resolved"
            error = "seeded delivery error" if is_error else ""
            labels = {"alertname": rule["alertname"]}
            notif_values.append(
                notif_line(ts, u, [rule["uid"]], [rule["folder"]], rule["receiver"],
                           status, error, labels))
            alert_values.append(
                alert_line(ts, u, rule["uid"], rule["folder"], rule["alertname"], status))

    # A single notification that references every rule at once. With the "any"
    # RBAC semantics, a user who can access just one of these rules should still
    # see this entry.
    if not args.no_mixed and len(args.rule) >= 2:
        ts = next_ts()
        u = str(uuidlib.uuid4())
        rule_uids = [r["uid"] for r in args.rule]
        folder_uids = [r["folder"] for r in args.rule]
        notif_values.append(
            notif_line(ts, u, rule_uids, folder_uids, "Shared receiver",
                       "firing", "", {"alertname": "MixedGroup"}))
        for idx, r in enumerate(args.rule):
            alert_values.append(
                alert_line(ts - timedelta(microseconds=idx + 1), u, r["uid"],
                           r["folder"], "MixedGroup", "firing"))

    streams = [
        {"stream": {"from": "notify-history-events"}, "values": notif_values},
        {"stream": {"from": "notify-history-alerts"}, "values": alert_values},
    ]

    if args.dry_run:
        print(json.dumps({"streams": streams}, indent=2))
        return 0

    push(args.loki_url, args.tenant, streams)

    print(f"Seeded {len(notif_values)} notification(s) and {len(alert_values)} alert(s) into "
          f"{args.loki_url}" + (f" (tenant {args.tenant})" if args.tenant else ""))
    for rule in args.rule:
        print(f"  rule uid={rule['uid']} folder={rule['folder']} receiver={rule['receiver']!r}")
    if not args.no_mixed and len(args.rule) >= 2:
        print("  + 1 mixed notification referencing all rules")
    return 0


if __name__ == "__main__":
    sys.exit(main())
