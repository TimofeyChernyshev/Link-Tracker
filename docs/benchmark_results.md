# 1000 ссылок

BenchmarkGetLinks_CacheVsNoCache/no_cache-16                  72          16216382 ns/op
BenchmarkGetLinks_CacheVsNoCache/with_cache-16               373           3661723 ns/op

SQL запрос с JOIN выполняется медленно, поэтому кэш выигрывает

# 100 ссылок

BenchmarkGetLinks_CacheVsNoCache/no_cache-16                 202           5131008 ns/op
BenchmarkGetLinks_CacheVsNoCache/with_cache-16               367           3267003 ns/op

# 10 ссылок

BenchmarkGetLinks_CacheVsNoCache/no_cache-16                 499           2081517 ns/op
BenchmarkGetLinks_CacheVsNoCache/with_cache-16               507           2825326 ns/op

Кэш не дает выигрыша из-за дополнительной операции сохранения данных в кэш + сериализация