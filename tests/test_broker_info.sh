#!/bin/bash

# Test script for broker info box feature
# This tests that the broker info box displays:
# - Broker count
# - Offline count  
# - ISR (In-Sync Replicas)
# - Out-of-sync replicas

echo "Testing Broker Info Box Feature"
echo "================================"
echo ""
echo "The broker info box should display on the right side of the brokers list with:"
echo "  ✓ Total brokers count"
echo "  ✓ Online/Offline status"
echo "  ✓ Controller node ID"
echo "  ✓ Total partitions across all topics"
echo "  ✓ Total replicas"
echo "  ✓ Under-replicated partitions"
echo "  ✓ Offline partitions (if any)"
echo ""
echo "To test:"
echo "1. Start kconduit: ./kconduit -b localhost:9092"
echo "2. Navigate to Brokers tab (use Tab key)"
echo "3. Verify the info box appears on the right side (30% width)"
echo "4. Check that all statistics are displayed correctly"
echo "5. The stats should update when brokers view is refreshed"
echo ""
echo "Expected layout:"
echo "┌─────────────────────────┐  ┌──────────────┐"
echo "│  Brokers Table (70%)    │  │ Info Box     │"
echo "│                         │  │ (30%)        │"
echo "│  ID | Host | Port ...   │  │              │"
echo "│  1  | ...               │  │ 📊 Status    │"
echo "│  2  | ...               │  │ Brokers: 3   │"
echo "│                         │  │ ✅ All Online│"
echo "│                         │  │              │"
echo "│                         │  │ 📈 Replicas  │"
echo "│                         │  │ Partitions:  │"
echo "│                         │  │ Replicas:    │"
echo "└─────────────────────────┘  └──────────────┘"