#!/bin/bash
# Copyright 2026 [Copyright Holder]
# Licensed under the Apache License, Version 2.0 (the "License");

set -e

# カバレッジ測定用の対象パッケージリストの生成
COVERPKG=$(go list ./internal/... | paste -sd, -)

# 全パッケージのテスト実行とカバレッジプロファイルの出力
echo "==> Running tests with coverage profile..."
go test -v -race -coverprofile=coverage.out -coverpkg="$COVERPKG" ./internal/... ./test/e2e

# Aerovoice コアビジネスロジック (ed137, channel, sip, pcap, media/codec, config) のステートメントカバー率を検証
echo "==> Verifying Aerovoice core protocol coverage (ed137, channel, sip, pcap, media/codec, config)..."
awk '
BEGIN { total = 0; covered = 0; }
/:/ {
    if ($0 ~ /\/internal\/(ed137|channel|sip|pcap|media\/codec|config)\//) {
        block = $1;
        stmts[block] = $2;
        if ($3 > max_count[block]) {
            max_count[block] = $3;
        }
    }
}
END {
    for (b in stmts) {
        total += stmts[b];
        if (max_count[b] > 0) {
            covered += stmts[b];
        }
    }
    if (total == 0) {
        print "ERROR: No statements found in Aerovoice core packages."
        exit 1
    }
    rate = (covered / total) * 100
    printf "=========================================\n"
    printf "Aerovoice Core Protocol Coverage Summary:\n"
    printf "  Covered Statements: %d\n", covered
    printf "  Total Statements:   %d\n", total
    printf "  Coverage Rate:      %.2f%%\n", rate
    printf "=========================================\n"
    if (rate < 80.0) {
        printf "ERROR: Aerovoice core coverage is %.2f%%, which is below the required 80.0%%!\n", rate
        exit 1
    }
    printf "SUCCESS: Aerovoice core coverage is %.2f%% (>= 80.0%%)\n", rate
}
' coverage.out

