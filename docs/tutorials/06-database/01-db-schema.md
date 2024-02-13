# Database Schema 

```
manifest {
    permissions: {}

    databases: {
        # define a local database named 'main1'
        main1: {
            resource: ldb://main1
            resolution-data: nil

            expected-schema-update: true
        }
    }
}

# The schema of an Inox Database is an object pattern and can be set by calling the `update_schema` method on the database.
# ⚠️ Calling `update_schema` requires the following property in the database description: `expected-schema-update: true`.

db = dbs.main1

pattern user = {
    name: str
}

print("schema before update:", db.schema)

db.update_schema(%{
    users: Set(user, #url)
}, {
    inclusions: :{
        %/users: []
    }
})

print("schema after update:", db.schema)
```