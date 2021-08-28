# LXD images

This repo holds build scripts and configs for Netsoc's custom LXD images.
[distrobuilder](https://distrobuilder.readthedocs.io/en/latest/) is used to actually build the images, but this is done
in GitHub Actions via a specially-crafted workflow. This is because the built images are actually hosted directly in
GitHub releases, with tags and assets in a specific format as accepted by
[OctoLXD](https://github.com/devplayer0/OctoLXD). LXD can then be configured to automatically update images through the
proxy.

## Updating images

To make a new release, create a tag of the form `<image_name>/v<version>-<rel>`, where `image_name` is the name of a
distrobuilder YAML file in `images/` (and optionally a shell script). **`version` needs to be
[semver](https://semver.org/) for it to be parsed correctly by OctoLXD.** Once the tag is pushed, the GitHub
`release.yaml` workflow will build the image for the `amd64` and `arm64` architectures and create the release.
