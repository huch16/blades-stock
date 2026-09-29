#!/usr/bin/env python3
"""
build_graph.py — P3.4: build-time graph manifest manager.

用途：
  1. 校验 .graph/graph.json 是否存在（不存在则退出非 0，build 失败）
  2. 写 .graph/manifest.json：
     - commit_sha: 当前 git HEAD（短 hash）
     - built_at: ISO8601 UTC
     - stale_after: built_at + 7 天
     - node_count / edge_count: 从 graph.json 读取
  3. P3.3 的 GraphQuery 工具会读 manifest.json 判断是否过期

参数：
  --root    仓库根路径（默认 cwd）
  --output  .graph 目录路径（默认 <root>/.graph）
  --commit  手动指定 commit SHA（默认 git rev-parse --short HEAD）
  --max-stale-days  过期阈值天数（默认 7）

退出码：
  0  成功
  1  graph.json 缺失
  2  graph.json 解析失败
  3  其它错误

P3.4 当前是 manifest-manager 实现：完整 graph builder 在 .graph/_internal/ 下，
后续可替换此脚本调用真正的 builder。
"""
import argparse
import datetime
import json
import os
import subprocess
import sys
from pathlib import Path


def get_git_short_sha(repo_root: Path) -> str:
    try:
        result = subprocess.run(
            ["git", "-C", str(repo_root), "rev-parse", "--short", "HEAD"],
            capture_output=True, text=True, timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except (subprocess.TimeoutExpired, FileNotFoundError):
        pass
    return "unknown"


def load_graph(graph_path: Path) -> dict:
    if not graph_path.exists():
        print(f"ERROR: {graph_path} 不存在", file=sys.stderr)
        print("  请先运行项目根的 graph builder（.graph/_internal/ 工具）生成图谱", file=sys.stderr)
        sys.exit(1)
    try:
        with open(graph_path, "r", encoding="utf-8") as f:
            return json.load(f)
    except json.JSONDecodeError as e:
        print(f"ERROR: {graph_path} 解析失败: {e}", file=sys.stderr)
        sys.exit(2)


def main() -> int:
    parser = argparse.ArgumentParser(description="build-time graph manifest manager")
    parser.add_argument("--root", default=".", help="仓库根路径")
    parser.add_argument("--output", default=None, help=".graph 目录路径（默认 <root>/.graph）")
    parser.add_argument("--commit", default=None, help="手动指定 commit SHA")
    parser.add_argument("--max-stale-days", type=int, default=7, help="过期阈值天数")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    graph_dir = Path(args.output).resolve() if args.output else root / ".graph"
    graph_json = graph_dir / "graph.json"

    g = load_graph(graph_json)

    node_count = len(g.get("nodes", []))
    edge_count = len(g.get("edges", []))

    commit_sha = args.commit
    if commit_sha is None:
        commit_sha = get_git_short_sha(root)

    built_at = datetime.datetime.now(datetime.timezone.utc)
    stale_after = built_at + datetime.timedelta(days=args.max_stale_days)

    manifest = {
        "commit_sha": commit_sha,
        "built_at": built_at.isoformat(),
        "stale_after": stale_after.isoformat(),
        "max_stale_days": args.max_stale_days,
        "node_count": node_count,
        "edge_count": edge_count,
        "graph_json": str(graph_json.relative_to(root)) if graph_json.is_relative_to(root) else str(graph_json),
        "schema_version": 1,
    }

    manifest_path = graph_dir / "manifest.json"
    with open(manifest_path, "w", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2, ensure_ascii=False)
        f.write("\n")

    print(f"OK: {manifest_path}")
    print(f"  commit: {commit_sha}")
    print(f"  nodes:  {node_count}")
    print(f"  edges:  {edge_count}")
    print(f"  built:  {built_at.isoformat()}")
    print(f"  stale:  {stale_after.isoformat()}")

    return 0


if __name__ == "__main__":
    sys.exit(main())