# Railway Google Sheets Credentials Troubleshooting Guide

## Issue: JSON Parsing Error
```
failed to parse credentials: json: cannot unmarshal string into Go value of type google.credentialsFile
```

## Root Cause
Railway environment variables containing JSON are often:
- Escaped (with `\n` and `\"` sequences)  
- Double-quoted (wrapped in outer quotes)
- Stringified (treated as string instead of JSON object)

## Solution Pattern

### 1. Enhanced Parsing Script
```bash
# Create credentials file with multiple format detection
if [[ "$GOOGLE_CREDENTIALS_JSON" == "{"* ]]; then
    # Raw JSON format
    echo "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
elif [[ "$GOOGLE_CREDENTIALS_JSON" == *"\\n"* ]] || [[ "$GOOGLE_CREDENTIALS_JSON" == *"\\"* ]]; then
    # Escaped JSON - decode escape sequences
    printf '%b' "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
else
    # Try removing outer quotes if present
    if [[ "$GOOGLE_CREDENTIALS_JSON" == '"'*'"' ]]; then
        CLEANED_JSON="${GOOGLE_CREDENTIALS_JSON:1:-1}"
        printf '%b' "$CLEANED_JSON" > "$CREDENTIALS_FILE"
    else
        echo "$GOOGLE_CREDENTIALS_JSON" > "$CREDENTIALS_FILE"
    fi
fi

# Always validate
if ! python3 -m json.tool "$CREDENTIALS_FILE" >/dev/null 2>&1; then
    echo "❌ Invalid JSON credentials"
    exit 1
fi
```

### 2. Debugging Commands
```bash
# Check first 50 chars to identify format
echo "🔍 Format: ${GOOGLE_CREDENTIALS_JSON:0:50}..."

# Test local validation
echo "$GOOGLE_CREDENTIALS_JSON" | python3 -m json.tool

# Test different parsing methods
printf '%b' "$GOOGLE_CREDENTIALS_JSON" > test_creds.json
python3 -m json.tool test_creds.json
```

### 3. Railway Environment Variable Best Practices

#### ✅ Correct Format (Raw JSON)
```json
{"type":"service_account","project_id":"your-project","private_key_id":"..."}
```

#### ❌ Problematic Formats
```bash
# Stringified JSON
"{\"type\":\"service_account\",\"project_id\":\"your-project\"}"

# Escaped JSON  
"{\n  \"type\": \"service_account\",\n  \"project_id\": \"your-project\"\n}"

# Base64 encoded
"ewogICJ0eXBlIjogInNlcnZpY2VfYWNjb3VudCIKfQ=="
```

## Implementation Checklist

### In startup script:
- [ ] Debug output showing credential format
- [ ] Multiple parsing approaches with fallbacks  
- [ ] JSON validation at each step
- [ ] Clear error messages for debugging
- [ ] Graceful degradation if credentials fail

### In Railway dashboard:
- [ ] Environment variable contains raw JSON (not stringified)
- [ ] No extra quotes around the JSON content
- [ ] No unnecessary escape sequences

### Testing:
- [ ] Local JSON validation passes
- [ ] Remote deployment shows correct parsing
- [ ] Google Sheets API calls succeed
- [ ] Error handling works for invalid credentials

## Common Fixes

### 1. Remove Outer Quotes
If Railway added quotes: `"{"type":"service_account"}"`
Remove them: `{"type":"service_account"}`

### 2. Decode Escape Sequences
If Railway escaped newlines: `{\n  "type": "service_account"\n}`
Use `printf '%b'` to decode properly

### 3. Base64 Alternative
If JSON parsing continues to fail, consider base64 encoding:
```bash
# Encode locally
cat credentials.json | base64 -w 0

# Decode in script  
echo "$BASE64_CREDENTIALS" | base64 -d > credentials.json
```

## Verification Commands
```bash
# Verify file was created correctly
ls -la credentials.json

# Validate JSON structure
python3 -m json.tool credentials.json

# Test Google API access
python3 -c "
from google.oauth2.service_account import Credentials
creds = Credentials.from_service_account_file('credentials.json')
print('✅ Credentials loaded successfully')
"
```

## Success Indicators
- ✅ `python3 -m json.tool credentials.json` passes
- ✅ No "cannot unmarshal string" errors in logs  
- ✅ Google Sheets API calls succeed
- ✅ WhatsApp drop data appears in spreadsheet

---
*Last updated: 2025-10-15*
*Status: Troubleshooting in progress - iterative improvements being deployed*