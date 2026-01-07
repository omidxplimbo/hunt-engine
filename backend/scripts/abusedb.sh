#!/bin/bash

domain=$1
if [ -z "$domain" ]; then
    echo "Usage: $0 <domain>"
    exit 1
fi

curl -s "https://www.abuseipdb.com/whois/$domain" \
     -H 'User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/114.0' \
     -H 'Cookie: XSRF-TOKEN=eyJpdiI6IlVqTHZxRmp1QTZYT2pzb0hMZHczRUE9PSIsInZhbHVlIjoiM1ZPVkxPYlZXNFhKODlPeU5PbTlTVTY4YVRxVWVxQ1JsU0tiOVhweUFicFFSR2NIZTE4ZVwvMWVtdHE3cFBhblUiLCJtYWMiOiI5MDJhNWI4NWJmNjQyMjYwZTY1MTEzMWNjZGQ0YTZjZWJhNWI5MWQyMTUwZjFhOTY3YTJjNDNhNzAwZjhlYTkxIn0%3D; abuseipdb_session=eyJpdiI6Ik4rYit4VlNFeEMyUitHa1BnMHpwOHc9PSIsInZhbHVlIjoibnFQMW55eEFIZFZsUzJxakxaWldsQVl6ekhvQ3IxNGxtNHZGcjV2YXMyUFcrclQ0OWxtcFdaRVlBMUlhZVlScyIsIm1hYyI6ImNlZWY2NjcyYmUxNDQ5ZWVkMjNhYjc0NjMzYzkwMWFjOWZlZWUwM2Q2MDQxNGMxNjEzZWY2OWUzNDViNjhjZGIifQ%3D%3D; env=hs3EkSYn1KFYarukaXPq' \
     | grep -E '<li>\w.*</li>' | sed -E "s/<\/?li>//g" | sed "s/$/.$domain/"
