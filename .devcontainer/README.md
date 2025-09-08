# Anubis Dev Container

Anubis offers a [development container](https://containers.dev/) image in order to make it easier to contribute to the project. This image is based on [Xe/devcontainer-base/go](https://github.com/Xe/devcontainer-base/tree/main/src/go), which is based on Debian Bookworm with the following customizations:

- [Fish](https://fishshell.com/) as the shell complete with a custom theme
- [Go](https://go.dev) at the most recent stable version
- [Node.js](https://nodejs.org/en) at the most recent stable version
- [Atuin](https://atuin.sh/) to sync shell history between your host OS and the development container
- [Docker](https://docker.com) to manage and build Anubis container images from inside the development container
- [Ko](https://ko.build/) to build production-ready Anubis container images
- [Neovim](https://neovim.io/) for use with Git

This development container is tested and known to work with [Visual Studio Code](https://code.visualstudio.com/). If you run into problems with it outside of VS Code, please file an issue and let us know what editor you are using.
