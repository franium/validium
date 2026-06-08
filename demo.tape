Output assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 900
Set Height 600
Set Theme "Catppuccin Mocha"
Set TypingSpeed 55ms
Set Padding 20

# --- setup (hidden) ---
Hide
Type `export PATH="$HOME/go/bin:$PATH"` Enter
Type `cd /tmp && rm -rf vd-demo && mkdir vd-demo && cd vd-demo` Enter
Type `printf 'APP_NAME=my-app\nPORT=8080\nDATABASE_URL=postgres://localhost/mydb\nDEBUG=true\nEMAIL=admin@example.com\n' > .env` Enter
Type `clear` Enter
Show

# --- 1. show the .env ---
Type `cat .env` Enter
Sleep 2s

# --- 2. init: generate schema ---
Sleep 500ms
Type `validium init` Enter
Sleep 2s

# --- 3. show generated schema ---
Type `cat validium.json` Enter
Sleep 3s

# --- 4. first check: all good ---
Type `clear` Enter
Type `validium check` Enter
Sleep 2.5s

# --- 5. generate .env.example ---
Type `validium generate` Enter
Sleep 1.5s
Type `cat .env.example` Enter
Sleep 2.5s

# --- 6. break a value in .env ---
Type `clear` Enter
Type `# Introduce a bad value...` Enter
Sleep 800ms
Type `sed -i 's/EMAIL=admin@example.com/EMAIL=not-an-email/' .env` Enter
Sleep 500ms
Type `cat .env` Enter
Sleep 2s

# --- 7. check fails ---
Type `validium check` Enter
Sleep 3s

# --- 8. fix the value ---
Type `sed -i 's/EMAIL=not-an-email/EMAIL=admin@example.com/' .env` Enter
Sleep 500ms

# --- 9. check passes again ---
Type `validium check` Enter
Sleep 3s

# --- 10. share secrets safely (encrypt / decrypt) ---
Type `clear` Enter
Type `# Share your .env safely with a teammate` Enter
Sleep 1s

# generate a local age keypair
Type `validium keygen` Enter
Sleep 2.5s

# encrypt .env for a recipient's public key -> .env.age
Type `validium encrypt $(sed -n 's/# public key: //p' .validium.key)` Enter
Sleep 2.5s

# the recipient decrypts .env.age back to .env with their identity
Type `validium decrypt .validium.key` Enter
Sleep 3s
