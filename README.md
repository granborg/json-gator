# json-gator

Aggregate and condition data from many different sources into customizable models on a single JSON web server. 

## Use Cases

1. Gathering data from many different sources to create a dashboard for monitoring operations.

2. Creating a modular backend for an application which has no web server, but can do HTTP POST requests.

3. Conditioning data to be fed into n8n or nodeRED via a web request for machine learning tasks, triggering events, sending emails, time series logging, etc.

4. Quickly creating and editing mock backend routes for simplified frontend development.

## Creating a Model

### Save a basic model to the web server

HTTP POST ```localhost:8080/model/living_room/thermostat/temp```

CONTENT
```json
{
    "current_f": 69,
    "ac_mode": "heat",
    "fan_mode": "on"
}
```

### Get the model to see your results

HTTP GET ```localhost:8080/model```

RESPONSE
```json
{
    "living_room": {
        "thermostat": {
            "temp": 
            {
                "current_f": 69,
                "ac_mode": "heat",
                "fan_mode": "on"
            }
        }
    }
}
```

### Narrow down on a specific section

HTTP GET ```localhost:8080/model/living_room/thermostat```

RESPONSE
```json
{
    "temp": 
    {
        "current_f": 69,
        "ac_mode": "heat",
        "fan_mode": "on"
    }
}
```

### Request a particular value

HTTP GET ```localhost:8080/model/living_room/thermostat/temp/ac_mode```

RESPONSE
```json
"heat"
```


## config.json File

This file contains the initial configuration for the server. It has three sections: model, nodes, and transformations.

You can use the route ```localhost:8080/config``` to update the initial configuration file.

You can also specify a custom file path for the config file using the environment variable ```CONFIG_FILE_PATH```.

## Model
The model section in the config file determines how the models are initially configured.

```json
"model": {
    "sales": {
        "north": 120000,
        "south": 85000,
        "east": 95000,
        "west": 110000
    },
    "costs": {
        "fixed": 75000,
        "variable": 180000,
        "marketing": 45000
    },
    "employees": {
        "count": 42,
        "avgSalary": 65000
    }
}
```

## Transformations: Performing data manipulation using JavaScript

The transformations section within the config file allows fields in the model to be extended with data transformations, allowing data type conversions and other calculations, or a customized presentation within the model.

The "implementation" field defines the JavaScript code. The output of this code is pushed into the JSON model based on the /-separated field name. The result will be converted into JSON, if possible. Otherwise, the string representation is used.

If the path from the field name is already in the model, it can be referred to within the implementation as a string, number, or JavaScript object with the variable name ```self```. For example, ```"self.toString + '%'"``` would convert the value to a string and append a percent sign.

You can also create values which are compositions of other values within the object, using the "parameters" section. In the configuration, the field names are the variable names within the implementation, and the values are the /-separated paths to the values within the model (similar to calling a function and passing in parameters).

### config.json Example
```json
"transformations": {
    "employees/avgSalary": {
        "implementation": "`$${self}.00`"
    },
    "sales/total": {
        "implementation": "north + south + east + west",
        "parameters": {
            "north": "sales/north",
            "south": "sales/south",
            "east": "sales/east",
            "west": "sales/west"
        }
    }
}
```
### Result (HTTP GET)

```json
 "employees": {
    "avgSalary": "$65000.00",
    "count": 42
},
"sales": {
    "east": 95000,
    "north": 120000,
    "south": 85000,
    "total": 410000,
    "west": 110000
}

```

## Nodes: Setting multiple fields to the same value at once

This is useful when you are representing the same data in different contexts, or with different transformations applied.

### config.json example
```json
"nodes": {
        "allSalesMetrics": [
            "sales/north",
            "sales/south",
            "sales/east",
            "sales/west"
        ]
    }
```

### Update all values at once
#### HTTP POST 
URL ```localhost:8080/node/allSales```

BODY ```10``` (just the number 10 yes)

#### HTTP GET 
URL ```localhost:8080/model/sales```
RESPONSE
```json
{
    "east": 10,
    "north": 10,
    "south": 10,
    "total": 40,
    "west": 10
}
```
## MQTT

json-gator also supports sending and receiving messages over MQTT.

The MQTT configuration contains connection parameters and an object containing the path mappings. The keys in this object represent the JSON paths in the model. The value is a list of objects that details how that path interacts with the MQTT broker.

topic - The topic to publish or subscribe to.

qos - MQTT QoS level: AtMostOnce (0), AtLeastOnce (1), ExactlyOnce (2).

retain - MQTT retain flag. Indicates whether the broker should make the value persistent.

publishType - Pub (0), Sub (1), PubSub (2). If Pub, json-gator will publish to the topic whenever the path changes. If Sub, the path will be changed whenever the topic changes to the value that was set over the MQTT connection. If PubSub, both behaviors happen at the same time.

```
"mqtt": {
        "broker": "mqtt://localhost:1883",
        "secure": false,
        "caCert": "ca.pem",
        "clientCert": "cert.pem",
        "clientKey": "cert.key",
        "caServerHostname": "example.com",
        "paths": {
            "sales": [
                {
                    "topic": "org/sales",
                    "qos": 0,
                    "retain": false,
                    "publishType": 0
                }
            ],
            "costs":  [
                {
                    "topic": "org/costs",
                    "publishType": 1
                }
            ],
            "employees": [
                {
                    "topic": "org/employees",
                    "qos": 0,
                    "retain": false,
                    "publishType": 2
                }
            ]
        }
    }
```