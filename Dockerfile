# 使用 OpenResty 官方镜像作为基础
FROM docker.1ms.run/openresty/openresty:alpine


# 安装依赖组件 (Luarocks、开发工具)
RUN apk add --no-cache gcompat

#RUN luarocks install lua-resty-http
#COPY ./lua_scripts/ /usr/local/openresty/nginx/lualib/

# 自定义 Nginx 配置
#COPY  raftstate /raftstate
#RUN cat /raftstate

COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf


# 指定工作目录
COPY ./bin/hraftd /usr/local/bin/

#COPY ./hraftd-service /etc/init.d/hraftd-service

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# 创建日志目录 (权限适配)
RUN mkdir -p /var/log/nginx/ \
    && chown -R nobody:nobody /var/log/nginx/

# 暴露端口
EXPOSE 80 10085 10086 443

# 启动命令
CMD ["/entrypoint.sh"]