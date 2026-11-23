"""U1: entry and build metadata always fresh, hashed assets immutable, missing assets404."""
import re

def run(c):
    code,html,_,headers=c.request('/',who=None)
    assert code==200
    headers={k.lower():v for k,v in headers.items()}
    assert 'no-store' in headers.get('cache-control',''),headers
    assert 'etag' not in headers
    for path in ['/','/index.html','/build-info.json']:
        code,body,_,headers=c.request(path,who=None,headers={'If-Modified-Since':'Wed, 21 Oct 2099 07:28:00 GMT'})
        headers={k.lower():v for k,v in headers.items()}
        assert code==200 and 'no-store' in headers.get('cache-control','') and 'etag' not in headers,(path,code,headers)
        if path.endswith('json'):assert re.fullmatch('[a-f0-9]{16}',body['build'])
    assets=re.findall(r'(?:src|href)="(/assets/[^\"]+)"',html)
    assert assets
    for path in assets:
        code,body,_,headers=c.request(path,who=None);headers={k.lower():v for k,v in headers.items()};assert code==200 and 'immutable' in headers.get('cache-control','')
    code,body,_,headers=c.request('/assets/missing-test.js',who=None)
    assert code==404 and '<script type="module"' not in body
    return {'entry':'200/no-store, conditional request not304','version':'public fingerprint/no-store','assets':'immutable; missing404'}
