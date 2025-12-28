if [ "$#" -eq "1" ]
then
    pushd sql/schema
    case "$1" in
        "up" | "down")
            goose postgres "postgres://postgres:postgres@localhost:5432/chirpy" "$1"
            ;;
        *)
            echo "Unknown argument: %1"
    esac
    popd
fi

sqlc generate && go build .
