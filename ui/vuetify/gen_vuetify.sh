API=/opt/homebrew/lib/node_modules/vuetify/dist/json/web-types.json
npm -g install vuetify@latest
find *.go | grep -v "fix-" | xargs rm
cat $API | vuetifyapi2go -comp=all
