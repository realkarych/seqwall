<a href="https://github.com/realkarych/seqwall">
<p align="center" width="100%">
    <img width="50%" alt="seqwall logo" src="https://github.com/user-attachments/assets/4ff7fce5-4e74-44ff-a6af-bb50d39449a3">
</p>
</a>

*<p align=center><a href="https://github.com/realkarych/seqwall">Seqwall</a> is a tool for PostgreSQL migrations testing</p>*

<hr>

## <p align=center>Installation</p>

### Homebrew (macOS & Linux)

```bash
brew tap realkarych/tap
brew install seqwall        # first install
brew upgrade seqwall        # later updates
```

### Debian / Ubuntu (APT)

```bash
# Import the GPG key
curl -fsSL https://realkarych.github.io/seqwall-apt/public.key \
  | sudo tee /etc/apt/trusted.gpg.d/seqwall.asc

# Add the repository
echo "deb [arch=$(dpkg --print-architecture)] \
  https://realkarych.github.io/seqwall-apt stable main" \
  | sudo tee /etc/apt/sources.list.d/seqwall.list

# Install / update
sudo apt update
sudo apt install seqwall          # first install
sudo apt upgrade  seqwall         # later updates
```

### Go install (Go ≥ 1.17)

```bash
go install github.com/realkarych/seqwall@latest
# make sure $GOBIN (default ~/go/bin) is on your PATH
```
