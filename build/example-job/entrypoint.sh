#!/bin/bash
set -euo pipefail

echo "================================"
echo "Switchyard Example Job"
echo "================================"
echo "Started at: $(date -Iseconds)"
echo "Job ID: ${SWITCHYARD_JOB_ID:-unknown}"
echo "Hostname: $(hostname)"
echo ""

# Simulate some work
echo "Processing data..."
for i in {1..5}; do
    echo "  Step $i/5"
    sleep 1
done

# Write outputs
echo ""
echo "Writing outputs..."

cat > /outputs/result.txt <<EOF
Job completed successfully!
Timestamp: $(date -Iseconds)
Hostname: $(hostname)
Job ID: ${SWITCHYARD_JOB_ID:-unknown}
EOF

# Create nested structure
mkdir -p /outputs/data
cat > /outputs/data/metrics.json <<EOF
{
  "status": "success",
  "items_processed": 42,
  "duration_seconds": 5,
  "timestamp": "$(date -Iseconds)"
}
EOF

cat > /outputs/data/summary.txt <<EOF
=== Job Summary ===
Status: SUCCESS
Items: 42
Duration: 5s
Completed: $(date -Iseconds)
EOF

echo ""
echo "Output files created:"
find /outputs -type f -exec ls -lh {} \;

echo ""
echo "Finished at: $(date -Iseconds)"
echo "================================"
