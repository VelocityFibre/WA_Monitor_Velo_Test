import os

# The simplest possible script.
# Read the variable and write it directly, with no processing.
cred_json_str = os.environ.get("GOOGLE_CREDENTIALS_JSON", "")
with open("credentials.json", "w") as f:
    f.write(cred_json_str)