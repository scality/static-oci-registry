- install nix `https://nixos.org/download/`
- install devenv
  ```bash
     nix-env --install --attr devenv -f https://github.com/NixOS/nixpkgs/tarball/nixpkgs-unstable
  ```

- install direnv (package / brew)
- add direnv hook `https://direnv.net/docs/hook.html`
- allow user for nix in config
  ```
  /etc/nix/nix.conf
  trusted-users = root <username>
  ```

- silence direnv optional (.config/direnv/direnv.toml )
```toml
[global]
log_filter="^loading"
```

- cd into dir, direnv allow 
