- Use vector<vector<int>> dp(n, vector<int>(n, 0)) for creating 2D vector quickly.
- Else only needs braces
```
class Solution {
public:
    int numPermsDISequence(string s) {
        int n = s.size() + 1;
        int mod = 1000000007;

        //dp[i][p] = how many permutation of length i+1 ending with p.
        vector<vector<int>> dp(n, vector<int>(n, 0));
        dp[0][0] = 1;

        //add new num(i) iteratively and count permutation ending 
        //with p = 0 to i         
        for(int i=1;i<n;i++){
            for(int p=0;p<i+1;p++){
                if(p < i){ 
                    //in previous perms, we replace p with i, every counting remains same. just prevp >=p should be seen as +1
                    if(s[i-1] == 'D'){
                        for(int prevp=0;prevp<i;prevp++)
                            if(prevp >= p)
                                dp[i][p] = (dp[i][p] + dp[i-1][prevp]) % mod;
                    }
                    else{
                        for(int prevp=0;prevp<i;prevp++){
                            if(prevp < p)
                                dp[i][p] = (dp[i][p] + dp[i-1][prevp]) % mod;
                        }
                    }
                }
                else if(p == i){
                    if(s[i-1] == 'I')
                        for(int prevp=0; prevp<i; prevp++){
                            dp[i][p] = (dp[i][p] + dp[i-1][prevp])%mod;
                        }
                }
            }
        }
        int ans =0;
        for(int p=0; p<n; p++)
            ans = (ans + dp[n-1][p]) % mod;
        return ans;
    }
};
```
