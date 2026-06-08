Output assets/demo.gif

Set Shell "bash"
Set FontSize 15
Set Width 900
Set Height 600
Set Theme "rose-pine"
Set TypingSpeed 55ms
Set Padding 20

# --- setup (hidden): create .env and bootstrap the schema with init ---
Hide
Type `export PATH="$HOME/go/bin:$PATH"` Enter
Type `cd /tmp && rm -rf vd-demo && mkdir vd-demo && cd vd-demo` Enter
Type `printf 'APP_NAME=my-app\nPORT=8080\nDATABASE_URL=postgres://localhost/mydb\nDEBUG=true\nEMAIL=admin@example.com\n' > .env` Enter
Type `validium init` Enter
Type `clear` Enter
Show

# --- 1. show the .env ---
Type `cat .env` Enter
Sleep 2s

# --- 2. add a new variable interactively ---
Type `validium add LOG_LEVEL` Enter
Sleep 2s

# pick the type: String (first option)
Enter
Sleep 1.2s

# required? -> Yes
Left
Sleep 600ms
Enter
Sleep 1s

# secret? -> No (default)
Enter
Sleep 1s

# does this string have allowed choices? -> Yes
Left
Sleep 600ms
Enter
Sleep 1.2s

# type the allowed choices
Type `debug,info,warn,error`
Sleep 1s
Enter
Sleep 2.5s

# --- 3. show the new variable in the schema ---
Type `clear` Enter
Type `cat validium.json` Enter
Sleep 3.5s

# --- 4. wire it into .env and generate the example ---
Type `clear` Enter
Type `echo 'LOG_LEVEL=info' >> .env` Enter
Sleep 500ms
Type `validium generate` Enter
Sleep 1.5s
Type `cat .env.example` Enter
Sleep 2.5s

# --- 5. check: all good ---
Type `clear` Enter
Type `validium check` Enter
Sleep 2.5s

# --- 6. break it with a value outside the allowed choices ---
Type `clear` Enter
Type `# LOG_LEVEL only accepts debug, info, warn or error...` Enter
Sleep 800ms
Type `sed -i 's/LOG_LEVEL=info/LOG_LEVEL=verbose/' .env` Enter
Sleep 500ms
Type `cat .env` Enter
Sleep 2s

# --- 7. check catches the invalid choice ---
Type `validium check` Enter
Sleep 3.5s

# --- 8. fix it ---
Type `sed -i 's/LOG_LEVEL=verbose/LOG_LEVEL=info/' .env` Enter
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
