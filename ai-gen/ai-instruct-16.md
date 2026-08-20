# AI instruct 16

## Build on IOS

I have a mac just add the `ci/deliver-ios.sh` for now doing the signing and everything.

I'll run it manually from time to time it will just do what cannot be achieved on the Linux gitlab-runner and source a `.env.ios` file you can prepare with all required environment variable.

The ipa will be pushed on git then the pipeline has to sign and add it in the nginx container `ui-and-mobile` stage (if the file exists).

I want it to use `VERSION` as reference for versioning and `mobile-release.keystore` for signing with the same variables defined in `ci/app/compute-env.sh`.

Then complete the wiki explaining how to test it (using testflight or something) util it'll be released on apple store.
