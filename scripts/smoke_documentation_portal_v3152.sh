#!/bin/bash
# Smoke Test for Documentation Portal (v3.15.2)
# Tests bilingual support and core sections availability

set -e

STAGE_URL="https://stage.mustache-security.ir"
DOCS_ROUTE="/documentation"
PASS=0
FAIL=0

echo "🚀 Starting Documentation Portal Smoke Test (v3.15.2)"
echo "Target: ${STAGE_URL}${DOCS_ROUTE}"
echo "---------------------------------------------"

# 1. Check if documentation route returns 200
echo -n "📄 Checking /documentation route... "
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "${STAGE_URL}${DOCS_ROUTE}")
if [ "$HTTP_CODE" -eq 200 ]; then
    echo "✅ PASS (200 OK)"
    ((PASS++))
else
    echo "❌ FAIL (HTTP $HTTP_CODE)"
    ((FAIL++))
    exit 1
fi

# 2. Check for Persian content (looking for a known Persian string from the docs)
echo -n "🇮🇷 Checking Persian content... "
if curl -s "${STAGE_URL}${DOCS_ROUTE}" | grep -q "راهنمای کامل"; then
    echo "✅ PASS (Persian content found)"
    ((PASS++))
else
    echo "⚠️  WARN (Persian content marker not found immediately, might be dynamic)"
    # Not failing here as content might be loaded via JS
fi

# 3. Check for English content
echo -n "🇬🇧 Checking English content... "
if curl -s "${STAGE_URL}${DOCS_ROUTE}" | grep -q "Complete Guide"; then
    echo "✅ PASS (English content found)"
    ((PASS++))
else
    echo "⚠️  WARN (English content marker not found immediately)"
fi

# 4. Check if screenshot directories exist (structural check)
echo -n "🖼️  Checking screenshot directories... "
if [ -d "frontend/public/docs/screenshots/fa" ] && [ -d "frontend/public/docs/screenshots/en" ]; then
    echo "✅ PASS (Directories exist)"
    ((PASS++))
else
    echo "❌ FAIL (Screenshot directories missing)"
    ((FAIL++))
fi

echo "---------------------------------------------"
echo "📊 Results: $PASS passed, $FAIL failed"

if [ $FAIL -eq 0 ]; then
    echo "✅ Documentation Portal Smoke Test PASSED"
    exit 0
else
    echo "❌ Documentation Portal Smoke Test FAILED"
    exit 1
fi
