- Use string(str.begin()+a, str.begin() + b) to extract substring.

```
class Solution {
public:
    string longestPrefix(string s) {
        int n=s.size();
        
        // longest matching prefix for string ending at i
        vector<int> lps (n, 0); 
        lps[0] = 0;
        int i=0, j=1, match = 0;
        while(j<n){
            if(s[i] == s[j]){
                match++;
                lps[j] = match;
                i++;j++;
            } else {
                if(match>0){
                    i = lps[match-1];
                    match = lps[match-1];
                } else {
                    if(s[0] != s[j])
                        lps[j++] = 0;
                }
            }
        }
        return string(s.begin(), s.begin() + lps[n-1]);
    }
};
```
