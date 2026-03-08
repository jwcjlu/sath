#!/usr/bin/env python3
"""从 stdin 读取每行一个数字，输出行数、和、均值。"""
import sys

def main():
    numbers = []
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            numbers.append(float(line))
        except ValueError:
            print(f"跳过非数字行: {line!r}", file=sys.stderr)
    if not numbers:
        print("未读到有效数字", file=sys.stderr)
        sys.exit(1)
    n = len(numbers)
    s = sum(numbers)
    print(f"行数: {n}, 和: {s}, 均值: {s/n}")

if __name__ == "__main__":
    main()
