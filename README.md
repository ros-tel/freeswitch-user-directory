Adding to autoload_configs/xml_curl.conf.xml
```
    <binding name="directory">
      <param name="gateway-url" value="http://127.0.0.1:8000/directory" bindings="directory"/>
    </binding>
```

Example set authentication data to Redis
```
SET agentid:1000 "{\"pass\":\"fea39eef503146089f8e8766fae3585a\",\"name\":\"FOO\",\"number\":\"31345134\"}"
```
