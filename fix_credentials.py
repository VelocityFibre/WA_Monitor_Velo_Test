
import os
import json

print("🐍 Starting credential fix script...")

# Get the environment variable
cred_json_str = os.environ.get("GOOGLE_CREDENTIALS_JSON")

if not cred_json_str:
    print("❌ ERROR: GOOGLE_CREDENTIALS_JSON environment variable not found.")
    exit(1)

print("✅ Found GOOGLE_CREDENTIALS_JSON environment variable.")

# Define the output file path
output_file = "credentials.json"

try:
    # First, parse the environment variable string into a Python object.
    # This correctly handles escaped characters and removes the outer quotes.
    cred_data = json.loads(cred_json_str)

    # Now, write the Python object back to the file as a properly formatted JSON object.
    with open(output_file, "w") as f:
        json.dump(cred_data, f, indent=2)

    print(f"✅ Successfully wrote credentials to {output_file}.")
    print("🐍 Credential fix script finished.")

except json.JSONDecodeError:
    print("❌ ERROR: The content of GOOGLE_CREDENTIALS_JSON is not valid JSON.")
    exit(1)
except Exception as e:
    print(f"❌ An unexpected error occurred: {e}")
    exit(1)
