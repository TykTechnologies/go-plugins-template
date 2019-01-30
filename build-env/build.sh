log() {
    printf "$@\n" >&2
}

error() {
    printf "[ERROR] $@\n" >&2
}

>&2 cat << EOF
                  ╲╲╲╲╲╲
                  ╲╲╲╲╲╲╲╲╲
           *******╲╲╲╲╲╲╲╲╲╲╲
          **********╲╲╲╲╲╲╲╲╲╲
       ***************╲╲╲╲╲╲╲╲╲╲
     ******************╲╲╲╲╲╲╲╲╲
    ********************╲╲╲╲╲╲╲╲╲
   ***********************╲╲╲╲╲╲╲╲
  *************************** ╲╲╲╲╲╲
  ****************************    ╲╲╲
  *****      *******      ****     _______      _        _
  ******      *****      *****    |__   __|    | |      (_)
  ****************************       | | _   _ | | __    _   ___ 
  *****************************      | || | | || |/ /   | | / _ \ 
  ******************************     | || |_| ||   <  _ | || (_) | 
 ********************************    |_| \__, ||_|\_\(_)|_| \___/ 
*********************************         __/ |
***  ************* *********  ****       |___/
      *********************
        *****************
         ******* ******
           ***** *****
          ****** ******
===========================================================================

EOF


source_path="/go/src/$PROJECT"
build_mode=

case "$1" in
"main")
  log "Trying to build main binary"
  build_mode=main
  ;;
"plugin")
  log "Trying to build plugin"
  build_mode=plugin
  ;;
*)
  error "Unknown build mode. First argument should be:\n  'main' - build main binary\n  'plugin' - build plugin. Require mounting '/plugin' folder with plugin source code\n"
  exit 1
  ;;
esac

printhelp(){
    log "Usage: (main|plugin) -v <branch-or-tag-or-sha1>\nExamples:\n  Build main binary: 'main -v v2.3.7'\n  Build plugin: 'plugin -v f874858'\n  Build from local folder: You can omit -v argument and build from local folder if '$source_path' folder is found, e.g. mounted inside docker.\n"
    exit 1
}

version=''
help=

OPTIND=2
while getopts ':v:h' flag; do
  case "${flag}" in
    h) help=1 ;;
    v) version="${OPTARG}" ;;
    \?) error "Unexpected option ${OPTARG}" && printhelp ;;
    :) error "Option -$OPTARG requires an argument." && printhelp ;;
  esac
done

if [ ! -z "$help" ]; then
    printhelp
fi

if [ -z "$version" ]; then
    if [ -d $source_path ]; then
        log "Found $source_path. Building based on mounted source code"
    else 
        error "Specify version which use to build binary or plugin, or mount $source_path folder\n"
        printhelp
    fi
else
    log "Specified version: $version"
fi

if [[ "$build_mode" == "plugin" ]]; then
    if [ ! -d /plugin ]; then
        error "/plugin folder not found. Ensure that you mounted plugin source inside docker container."
        printhelp
   fi
fi

cd $source_path

go get -d -v ./...

if [[ "$build_mode" == "main" ]]; then
    go build -o binary ./
    cat ./binary
else
    plugin_files=`ls /plugin/*.go | tr "\n" " "`
    build_target=""
    for file in $line; do
        cp /plugin/$(file) ./plugin_$(file)
        build_target="$build_target plugin_$(file)"
    done
    cp -n -r /plugin/vendor/* ./vendor/*

    CGO_ENABLED=0 go build -buildmode=plugin -o plugin.so $build_target
    cat ./plugin.so
fi
