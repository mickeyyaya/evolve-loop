#!/usr/bin/env python3
import json
import sys
import os
import argparse

def get_state(filepath=".evolve/state.json"):
    if not os.path.exists(filepath):
        return {"error": "State file not found"}
    try:
        with open(filepath, 'r') as f:
            return json.load(f)
    except Exception as e:
        return {"error": str(e)}

def main():
    parser = argparse.ArgumentParser(description="Evolve Loop Status")
    parser.add_argument("--json", action="store_true", help="Output in JSON format")
    args = parser.parse_args()

    state = get_state()
    
    if args.json:
        print(json.dumps(state, indent=2))
        sys.exit(0)

    if "error" in state:
        print(f"Error: {state['error']}")
        sys.exit(1)

    print("=== Evolve Loop Status ===")
    print(f"Last Cycle: {state.get('lastCycleNumber', 'N/A')}")
    print(f"Strategy: {state.get('strategy', 'N/A')}")
    print(f"Mastery Level: {state.get('mastery', {}).get('level', 'N/A')}")
    print(f"Instincts: {state.get('instinctCount', 0)}")
    
    benchmark = state.get("projectBenchmark", {})
    print(f"\nProject Benchmark: {benchmark.get('overall', 'N/A')}")

if __name__ == "__main__":
    main()
