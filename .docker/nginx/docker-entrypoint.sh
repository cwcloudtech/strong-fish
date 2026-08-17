#!/bin/sh
set -e

# The frontend is built once with ${SF_API_URL}-style placeholders left in the
# bundle (see sf-ui/.env.react), so one image can be pointed at any deployment
# by substituting them at container start rather than rebuilding per environment.
if [ "$1" = "nginx" ]; then
    defined_envs=$(printf '${%s} ' $(env | cut -d= -f1))
    front_app_path="/usr/share/nginx/html"

    for file in $(find "${front_app_path}" -type f \( -name '*.css' -o -name '*.js' -o -name '*.html' \))
    do
        envsubst "$defined_envs" < "$file" > "${file}.tmp"
        mv "${file}.tmp" "$file"
    done

    # Bust any cached bundle from a previous image on the same host.
    sed -i 's!\(main\)\.\([^\.]\+\)\.\(js\|css\)"!\1.\2.\3?t='"$(date '+%s')"'"!g' "${front_app_path}/index.html"
fi

exec "$@"
