# Data Formats for PIXLISE

## What is this?

This repository contains the protobuf files that describe the structure of the binary formats used by Pixlise. It was originally a part of the pixlise repository, but it was broken into separate repositories for client vs api, and we're also going to use this format in Piquant, so it makes sense to have this separated out.

## Files

`experiment.proto` describes the format of the "Dataset" files used by Pixlise. These include:
- All spectra (tactical just includes bulk sum & max for each detector), indexed by PMC
- Beam location information for each PMC
- Metadata for each PMC and for the overall file
- Context image file names
- Name of detector configuration used for recording the spectrum samples
- If it's a tactical file it can include pseudointensity data too.

`quantification.proto` is basically a binary version of a Piquant MAP CSV file. Storing these as binary generally is smaller than the textual CSV (not always!) and also the data is accessible as integers/floats by the client without needing to string-convert from CSV.

## Use as Submodules

This repository is intended to be used as a submodule in pixlise client, pixlise api and piquant repos. See:
https://git-scm.com/book/en/v2/Git-Tools-Submodules
