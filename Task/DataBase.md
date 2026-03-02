## 方案设计

### 创建（打开）数据库 - 根方法
参数：
- 数据库类型
- 数据库路径（绝对路径前面是'/'，相对路径没有；memory类型不需要使用这个参数）
- 数据库额外配置（可以不传，包含数据库文件模式、全同步等配置信息）
返回值：
- 正常返回DbConnection，否则返回空

### 删除数据库 - 根方法
参数：
- 数据库路径
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 关闭数据库 - DbConnection中的方法
参数：
- 关闭时的配置（例如强制同步数据库文件、强制同步数据库文件所在目录等）
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 创建数据表 - DbConnection中的方法
参数：
- 数据表名
- Vector<FieldDescription> table_desc （FieldDescription包含名字、数据类型、是否主键、是否自增、是否有默认值、默认值等信息，如果多个主键要abort）
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 删除数据表 - DbConnection中的方法
参数：
- 数据表名
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 更新数据表 - DbConnection中的方法
参数：
- 数据表名
- Vector<Str> remove_field （如果移除主键要报错）
- Vector<FieldUpdateDescription> update_field （FieldDescription包含原名字、新名字等信息）
- Vector<FieldDescription> new_field （如果包含主键要报错）
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 插入数据 - DbConnection中的方法
参数：
- 数据表名
- Vector<FieldDate> data (包含了名字、数据)
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 更新数据 - DbConnection中的方法
参数：
- 数据表名
- Vector<FieldDate> data (包含了名字、数据)
- FieldId （包含了id所在的列名、还有id）
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 删除数据 - DbConnection中的方法
参数：
- 数据表名
- FieldId （包含了id所在的列名、还有id）
返回值：
- 错误代码，0表示成功；其他代码表示对应的错误

### 查询数据 - DbConnection中的方法
参数：
- 数据表名
- FieldId （包含了id所在的列名、还有id）
- 查询条数
- 排序依据 Vector<OrderDescript> 可以包含多列排序，并且可以指定升序还是降序
返回值：
- Vector<FieldDate> data；否则返回空

### 辅助函数集 - 根方法
将基本类型转换为对应的Sqlite SQL类型（全部是Str的格式）
从Sqlite类型，转换成Rust类型